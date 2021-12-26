package main

import (
	"math/rand"
	"time"

	"github.com/faiface/beep"
	"github.com/faiface/beep/mp3"
	"github.com/faiface/beep/speaker"
)

const defaultSampleRate beep.SampleRate = 48000

type playbackMode int

const (
	normal playbackMode = iota
	repeat
	repeatOne
	random
)

func (mode playbackMode) String() string {
	return [4]string{"normal", "repeat", "repeat one", "random"}[mode]
}

type playbackStatus int

const (
	stopped playbackStatus = iota
	playing
	paused
	seekBWD
	seekFWD
	skipBWD
	skipFWD
)

// □ ▹ ▯▯ ◃◃ ▹▹ ▯◃ ▹▯
// fancy stuff, but doesn't work everywhere
func (status playbackStatus) String() string {
	return [7]string{" \u25a1", " \u25b9", "\u25af\u25af",
		"\u25c3\u25c3", "\u25b9\u25b9", "\u25af\u25c3",
		"\u25b9\u25af"}[status]
}

type playback struct {
	currentTrack int
	totalTracks  int
	stream       *mediaStream
	format       beep.Format
	timeStep     time.Duration

	status         playbackStatus
	bufferedStatus playbackStatus
	playbackMode   playbackMode
	volume         float64
	muted          bool
}

func (player *playback) raiseVolume() {
	player.volume += 0.5

	if player.volume > 0.0 {
		player.volume = 0.0
	}

	player.muted = false

	player.setVolume()
}

func (player *playback) lowerVolume() {
	player.volume -= 0.5
	if player.volume < -10.0 {
		player.volume = -10.0
	}

	if player.volume < -9.6 {
		player.muted = true
	}

	player.setVolume()
}

func (player *playback) setVolume() {
	if player.isReady() {
		speaker.Lock()
		player.stream.volume.Silent = player.muted
		player.stream.volume.Volume = player.volume
		speaker.Unlock()
	}
}

func (player *playback) mute() {
	player.muted = !player.muted
	if player.isReady() {
		speaker.Lock()
		player.stream.volume.Silent = player.muted
		speaker.Unlock()
	}
}

func (player *playback) seek(forward bool) bool {
	if !player.isPlaying() || !player.isPlaying() {
		return false
	}

	var pos = player.format.SampleRate.N(time.Second * player.timeStep)
	var newStatus playbackStatus

	if forward {
		newStatus = seekFWD
	} else {
		newStatus = seekBWD
		pos *= -1
	}
	player.bufferStatus(newStatus)

	speaker.Lock()
	newPos := player.stream.streamer.Position()
	newPos += pos

	if newPos < 0 {
		newPos = 0
	}

	if newPos >= player.stream.streamer.Len() {
		// FIXME: crashes when seeking past ending of the last track
		// if offset from end to -5, it doesn't crash
		// crashes only on len -1, -2, -3, -4
		// -5 - or less fine, 0 - fine, but errors with EOF
		// crashes with index out of range in beep sources
		newPos = player.stream.streamer.Len()
	}

	if err := player.stream.streamer.Seek(newPos); err != nil {
		// FIXME:
		// sometimes reports errors, for example this one:
		// https://github.com/faiface/beep/issues/116
		// causes track to skip, again, only sometimes
		// NOTE: ignore EOF entirely, jumping straight to the end
		// doesn't seem to break anything and fixes crashes described
		// above, all other errors just reported on screen, nothing
		// could be done about them (?)
		if err.Error() != "mp3: EOF" {
			window.sendEvent(newErrorMessage(err))
		}
	}
	speaker.Unlock()

	return true
}

func (player *playback) resetPosition() {
	if player.isReady() {
		window.sendEvent(newDebugMessage("reset position"))
		speaker.Lock()
		if err := player.stream.streamer.Seek(0); err != nil {
			window.sendEvent(newErrorMessage(err))
		}
		speaker.Unlock()
	}
}

func (player *playback) getCurrentTrackPosition() time.Duration {
	if player.isReady() {
		speaker.Lock()
		position := player.format.SampleRate.D(player.stream.
			streamer.Position()).Round(time.Second)
		speaker.Unlock()
		return position
	}
	return 0
}

// play/pause/seekFWD/seekBWD count as active state
func (player *playback) isPlaying() bool {
	/* if player.status == playing || player.status == paused ||
		player.status == seekFWD || player.status == seekBWD {
		return player.isReady()
	}
	return false */
	return player.status == playing || player.status == paused ||
		player.status == seekFWD || player.status == seekBWD
}

func (player *playback) isReady() bool {
	return player.stream != nil
}

func (player *playback) skip(forward bool) bool {
	if player.totalTracks == 0 {
		return false
	}

	if player.playbackMode == random {
		player.nextTrack()
		return true
	}

	window.sendEvent(newDebugMessage("skip track"))
	player.stop()
	player.clearStream()

	if forward {
		player.currentTrack = (player.currentTrack + 1) %
			player.totalTracks
		player.status = skipFWD
	} else {
		player.currentTrack = (player.totalTracks +
			player.currentTrack - 1) %
			player.totalTracks
		player.status = skipBWD
	}

	// TODO: remove after response reading cancellation is implemented
	go player.delaySwitching()
	return true
}

func (player *playback) nextMode() {
	player.playbackMode = (player.playbackMode + 1) % 4
}

func (player *playback) nextTrack() {
	window.sendEvent(newDebugMessage("next track"))
	switch player.playbackMode {

	case random:
		var previousTrack int

		if player.totalTracks > 1 {
			rand.Seed(time.Now().UnixNano())
			previousTrack = player.currentTrack
			// never play same track again if random
			for player.currentTrack == previousTrack {
				player.currentTrack = rand.Intn(player.totalTracks)
			}
		}
		player.stop()

		if player.currentTrack >= previousTrack {
			player.status = skipFWD
		} else {
			player.status = skipBWD
		}

		player.clearStream()
		go player.delaySwitching()

	// beep does have loop one, but stream should be set
	// up to loop from the very start to play indefinetly,
	// which is not ideal
	case repeatOne:
		// doesn't work without position reset?
		player.resetPosition()
		player.restart()

	case repeat:
		player.skip(true)

	case normal:
		if player.currentTrack == player.totalTracks-1 {
			// prepare new stream for playback again
			// and immediately stop it
			player.restart()
			player.stop()
			return
		}
		player.skip(true)
	}
}

// to prevent downloading every item on fast track switching
// probably dumb idea
func (player *playback) delaySwitching() {
	track := player.currentTrack
	time.Sleep(time.Second / 2)
	if track == player.currentTrack {
		window.sendEvent(newTrack(player.currentTrack))
	}
}

func (player *playback) setTrack(track int) {
	player.stop()
	player.clearStream()
	if track >= player.currentTrack {
		player.status = skipFWD
	} else {
		player.status = skipBWD
	}
	player.currentTrack = track
	go player.delaySwitching()
}

func (player *playback) play(key string) {
	//player.clear()
	//player.currentTrack = track
	streamer, format, err := mp3.Decode(wrapInRSC(key))
	if err != nil {
		window.sendEvent(newErrorMessage(err))
		return
	}
	speaker.Lock()
	player.format = format
	player.stream = newStream(format.SampleRate, streamer, player.volume, player.muted)
	player.status = playing
	speaker.Unlock()
	window.sendEvent(newDebugMessage("playback started"))
	// deadlocks if anything speaker related is done inside callback
	// since it's locking device itself
	speaker.Play(beep.Seq(player.stream.volume, beep.Callback(
		func() {
			//window.sendEvent(newDebugMessage("next track callback"))
			// go player.nextTrack()
			window.sendEvent(&eventNextTrack{})
			//window.sendEvent(newDebugMessage("next track callback exit"))
		})))
}

func (player *playback) restart() {
	window.sendEvent(newDebugMessage("restart playback"))
	speaker.Clear()
	speaker.Play(beep.Seq(player.stream.volume, beep.Callback(
		func() {
			//window.sendEvent(newDebugMessage("restart callback"))
			//go player.nextTrack()
			//window.sendEvent(newDebugMessage("restart callback exit"))
			window.sendEvent(&eventNextTrack{})
		})))
	player.status = playing
}

// stop is actually pause with position reset
func (player *playback) stop() {
	window.sendEvent(newDebugMessage("playback stopped"))
	player.status = stopped
	if player.isReady() {
		player.resetPosition()
		speaker.Lock()
		player.stream.ctrl.Paused = true
		speaker.Unlock()
	}
}

func (player *playback) clearStream() {
	window.sendEvent(newDebugMessage("clearing buffer"))
	speaker.Clear()
	if player.isReady() {
		speaker.Lock()
		err := player.stream.streamer.Close()
		player.stream = nil
		speaker.Unlock()
		if err != nil {
			window.sendEvent(newErrorMessage(err))
		}
	}
}

func (player *playback) bufferStatus(newStatus playbackStatus) {
	if player.status != seekBWD && player.status != seekFWD {
		player.bufferedStatus = player.status
		player.status = newStatus
	}
}

func (player *playback) playPause() bool {
	if !player.isReady() {
		return false
	}

	if player.status == seekBWD || player.status == seekFWD {
		player.status = player.bufferedStatus
	}

	switch player.status {

	case paused, stopped:
		player.status = playing

	case playing:
		player.status = paused

	default:
		return false

	}

	speaker.Lock()
	player.stream.ctrl.Paused = !player.stream.ctrl.Paused
	speaker.Unlock()

	return true
}

// device initialization
func init() {
	sr := beep.SampleRate(defaultSampleRate)
	speaker.Init(sr, sr.N(time.Second/10))
}
