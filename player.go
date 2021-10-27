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

	status         playbackStatus
	bufferedStatus playbackStatus
	playbackMode   playbackMode
	volume         float64
	muted          bool
}

func (player *playback) changePosition(pos int) time.Duration {
	if !player.isPlaying() {
		return 0
	}
	newPos := player.stream.streamer.Position()
	newPos += pos
	if newPos < 0 {
		newPos = 0
	}
	if newPos >= player.stream.streamer.Len() {
		// FIXME: crashes when seeking past ending of the last track
		// if offset from end, it doesn't crash
		// probably triggers second nexttrack() from different goroutine
		// really janky solution
		newPos = player.stream.streamer.Len() - 5
	}
	if err := player.stream.streamer.Seek(newPos); err != nil {
		// FIXME:
		// sometimes reports errors, for example this one:
		// https://github.com/faiface/beep/issues/116
		// causes track to skip, again, only sometimes
		window.sendInterruptEvent(err)
	}
	return player.format.SampleRate.D(newPos).Round(time.Second)
}

func (player *playback) resetPosition() {
	if player.isReady() {
		window.sendInterruptEvent(eventDebugMessage("reset position"))
		if err := player.stream.streamer.Seek(0); err != nil {
			window.sendInterruptEvent(err)
		}
	}
}

func (player *playback) getCurrentTrackPosition() time.Duration {
	if player.isReady() {
		speaker.Lock()
		defer speaker.Unlock()
		return player.format.SampleRate.D(player.stream.
			streamer.Position()).Round(time.Second)
	}
	return 0
}

func (player *playback) isPlaying() bool {
	if player.stream != nil && (player.status == playing ||
		player.status == paused || player.status == seekFWD ||
		player.status == seekBWD) {
		return true
	}
	return false
}

func (player *playback) isReady() bool {
	return player.stream != nil
}

func (player *playback) skip(forward bool) {
	if player.totalTracks == 0 {
		return
	}
	window.sendInterruptEvent(eventDebugMessage("skip track"))
	if player.playbackMode == random {
		player.nextTrack()
		return
	}
	player.stop()
	player.clear()
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

}

func (player *playback) nextTrack() {
	window.sendInterruptEvent(eventDebugMessage("next track"))
	switch player.playbackMode {

	case random:
		rand.Seed(time.Now().UnixNano())
		player.currentTrack = rand.Intn(player.totalTracks)
		go player.delaySwitching()

	case repeatOne:
		//window.sendPlayerEvent(eventNextTrack(player.currentTrack))
		player.resetPosition()
		player.restart()
		return

	case repeat:
		player.skip(true)

	case normal:
		if player.currentTrack == player.totalTracks-1 {
			player.restart()
			player.stop()
			return
		}
		player.skip(true)
	}
	player.stop()
	player.clear()
	player.status = skipFWD
}

func (player *playback) delaySwitching() {
	// to prevent downloading every item on fast track switching
	// probably dumb idea
	track := player.currentTrack
	time.Sleep(time.Second / 2)
	if track == player.currentTrack {
		window.sendInterruptEvent(eventNextTrack(player.currentTrack))
	}
}

func (player *playback) play(track int, key eventTrackDownloader) {
	// TODO: update current track for playlist view
	// FIXME: it is possible to play two first tracks at the same time, if you input
	// two queries fast enough, this stops previous playback
	//player.stop()
	window.sendInterruptEvent(eventDebugMessage("music play"))
	streamer, format, err := mp3.Decode(wrapInRSC(key))
	if err != nil {
		window.sendInterruptEvent(err)
		return
	}
	speaker.Lock()
	player.format = format
	player.stream = newStream(format.SampleRate, streamer, player.volume, player.muted)
	player.status = playing
	speaker.Unlock()
	// deadlocks if anything speaker related is done inside callback
	// since it's locking device itself
	speaker.Play(beep.Seq(player.stream.volume, beep.Callback(
		func() {
			go player.nextTrack()
		})))
}

func (player *playback) restart() {
	speaker.Clear()
	speaker.Play(beep.Seq(player.stream.volume, beep.Callback(
		func() {
			go player.nextTrack()
		})))
	player.status = playing
}

func (player *playback) stop() {
	window.sendInterruptEvent(eventDebugMessage("music stopped"))
	player.status = stopped
	if player.isReady() {
		player.resetPosition()
		speaker.Lock()
		player.stream.ctrl.Paused = true
		speaker.Unlock()
	}
}

func (player *playback) clear() {
	speaker.Clear()
	if player.isReady() {
		speaker.Lock()
		// FIXME: doesn't really matter if we close streamer or not
		// method close in underlying data does absolutely nothing
		err := player.stream.streamer.Close()
		speaker.Unlock()
		if err != nil {
			window.sendInterruptEvent(err)
		}
	}
}

func (player *playback) bufferStatus(newStatus playbackStatus) {
	if player.status != seekBWD && player.status != seekFWD {
		player.bufferedStatus = player.status
		player.status = newStatus
	}
}

func (player *playback) initPlayer() {
	window.sendInterruptEvent(eventDebugMessage("player reset"))
	// keeps volume and playback mode from previous playback
	*player = playback{playbackMode: player.playbackMode, volume: player.volume,
		muted: player.muted}
}

func (player *playback) handleEvent(key rune) bool {
	switch key {
	// TODO: change controls
	case ' ':
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

	case 'a', 'A':
		if !player.isPlaying() {
			return false
		}
		player.bufferStatus(seekBWD)
		speaker.Lock()
		player.changePosition(-player.format.SampleRate.N(time.Second * 3))
		speaker.Unlock()

	case 'd', 'D':
		if !player.isPlaying() {
			return false
		}
		player.bufferStatus(seekFWD)
		speaker.Lock()
		player.changePosition(player.format.SampleRate.N(time.Second * 3))
		speaker.Unlock()

	case 's', 'S':
		if !player.isReady() || player.volume < -9.6 {
			return false
		}
		player.volume -= 0.5
		speaker.Lock()
		if !player.muted && player.volume < -9.6 {
			player.muted = true
			player.stream.volume.Silent = player.muted
		}
		player.stream.volume.Volume = player.volume
		speaker.Unlock()

	case 'w', 'W':
		if !player.isReady() || player.volume > -0.4 {
			return false
		}
		player.volume += 0.5
		speaker.Lock()
		if player.muted {
			player.muted = false
			player.stream.volume.Silent = player.muted
		}
		player.stream.volume.Volume = player.volume
		speaker.Unlock()

	case 'm', 'M':
		if !player.isReady() {
			return false
		}
		player.muted = !player.muted
		speaker.Lock()
		player.stream.volume.Silent = player.muted
		speaker.Unlock()

	case 'r', 'R':
		player.playbackMode = (player.playbackMode + 1) % 4

	case 'b', 'B':
		if player.getCurrentTrackPosition() > time.Second*3 {
			speaker.Lock()
			player.resetPosition()
			speaker.Unlock()
		} else {
			player.skip(false)
		}

	case 'f', 'F':
		player.skip(true)

	case 'p', 'P':
		player.stop()
		player.status = stopped

	default:
		return false
	}
	// nil = just update text on screen
	window.sendInterruptEvent(nil)
	return true
}

// device initialization
func init() {
	sr := beep.SampleRate(defaultSampleRate)
	speaker.Init(sr, sr.N(time.Second/10))
}
