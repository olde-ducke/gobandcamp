package main

import (
	"bytes"
	"fmt"
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

var modes = [4]string{"normal", "repeat", "repeat one", "random"}

func (mode playbackMode) String() string {
	return modes[mode]
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

var statuses = [7]string{" \u25a1", " \u25b9", "\u25af\u25af",
	"\u25c3\u25c3", "\u25b9\u25b9", "\u25af\u25c3",
	"\u25b9\u25af"}

// □ ▹ ▯▯ ◃◃ ▹▹ ▯◃ ▹▯
// fancy stuff, but doesn't work everywhere
func (status playbackStatus) String() string {
	return statuses[status]
}

type beepPlayer struct {
	currentTrack int
	totalTracks  int // TODO: remove
	stream       *mediaStream
	format       beep.Format
	timeStep     time.Duration

	status         playbackStatus
	bufferedStatus playbackStatus
	playbackMode   playbackMode
	volume         float64
	muted          bool

	// for debug reporting
	dbg func(string)

	// TODO remove
	// text chan<- interface{}
}

func newBeepPlayer(dbg func(string)) *beepPlayer {
	initializeDevice()
	return &beepPlayer{timeStep: 2, dbg: dbg}
}

// device initialization
func initializeDevice() {
	// TODO: add setting sample rate
	sr := beep.SampleRate(defaultSampleRate)
	speaker.Init(sr, sr.N(time.Second/10))
}

func (player *beepPlayer) raiseVolume() {
	player.volume += 0.5

	if player.volume > 0.0 {
		player.volume = 0.0
	}

	player.muted = false

	player.setVolume()
}

func (player *beepPlayer) lowerVolume() {
	player.volume -= 0.5
	if player.volume < -10.0 {
		player.volume = -10.0
	}

	if player.volume < -9.6 {
		player.muted = true
	}

	player.setVolume()
}

func (player *beepPlayer) setVolume() {
	if player.isReady() {
		speaker.Lock()
		player.stream.volume.Silent = player.muted
		player.stream.volume.Volume = player.volume
		speaker.Unlock()
	}
}

func (player *beepPlayer) mute() {
	player.muted = !player.muted
	if player.isReady() {
		speaker.Lock()
		player.stream.volume.Silent = player.muted
		speaker.Unlock()
	}
}

func (player *beepPlayer) getVolume() string {
	if player.muted {
		return "mute"
	} else {
		return fmt.Sprintf("%4.0f", (100 + player.volume*10))
	}
}

func (player *beepPlayer) getCurrentTrack() int {
	if player.currentTrack < 0 {
		player.currentTrack = 0
	}

	if player.currentTrack > player.totalTracks-1 {
		player.currentTrack = player.totalTracks - 1
	}

	return player.currentTrack
}

func (player *beepPlayer) getPlaybackMode() string {
	return player.playbackMode.String()
}

func (player *beepPlayer) getStatus() playbackStatus {
	if player.bufferedStatus < 0 {
		return player.status
	}
	status := player.bufferedStatus
	player.bufferedStatus = -1
	return status
}

func (player *beepPlayer) seek(forward bool) (bool, error) {
	if !player.isPlaying() {
		return false, nil
	}

	var pos = player.format.SampleRate.N(time.Second * player.timeStep)

	if forward {
		player.bufferedStatus = seekFWD
	} else {
		player.bufferedStatus = seekBWD
		pos *= -1
	}

	speaker.Lock()
	newPos := player.stream.streamer.Position()
	newPos += pos

	if newPos < 0 {
		newPos = 0
	}

	if newPos >= player.stream.streamer.Len() {
		// FIXME: crashes when seeking past ending of the last track,
		// if offset from end to -5, it doesn't crash
		// crashes only on len -1, -2, -3, -4
		// -5 - or less fine, 0 - fine, but errors with EOF,
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
			return false, err
		}
	}
	speaker.Unlock()

	return true, nil
}

// FIXME: doesn't return error
func (player *beepPlayer) resetPosition() {
	//	if player.isReady() {
	if player.isReady() {
		player.dbg("reset position")
		speaker.Lock()
		if err := player.stream.streamer.Seek(0); err != nil {
			player.dbg(err.Error())
		}
		speaker.Unlock()
	}
}

func (player *beepPlayer) getCurrentTrackPosition() time.Duration {
	if player.isReady() {
		speaker.Lock()
		position := player.format.SampleRate.D(player.stream.streamer.Position())
		speaker.Unlock()
		return position
	}
	return 0
}

// play/pause/seekFWD/seekBWD count as active state
func (player *beepPlayer) isPlaying() bool {
	return player.status == playing || player.status == paused
}

func (player *beepPlayer) isReady() bool {
	return player.stream != nil
}

func (player *beepPlayer) skip(forward bool) bool {
	if player.totalTracks == 0 {
		return false
	}

	if player.playbackMode == random {
		player.nextTrack()
		return true
	}

	player.dbg("skip track")
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

	return true
}

func (player *beepPlayer) nextMode() {
	player.playbackMode = (player.playbackMode + 1) % 4
}

func (player *beepPlayer) nextTrack() {
	player.dbg("next track")
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

func (player *beepPlayer) setTrack(track int) bool {
	if track >= player.totalTracks || track < 0 || track == player.currentTrack {
		return false
	}

	player.stop()
	player.clearStream()
	if track >= player.currentTrack {
		player.status = skipFWD
	} else {
		player.status = skipBWD
	}
	player.currentTrack = track
	return true
}

func (player *beepPlayer) play(data []byte) error {
	//player.clear()
	//player.currentTrack = track
	reader := bytes.NewReader(data)
	streamer, format, err := mp3.Decode(NopSeekCloser(reader))
	if err != nil {
		return err
	}
	speaker.Lock()
	player.format = format
	player.stream = newStream(format.SampleRate, streamer, player.volume, player.muted)
	player.status = playing
	speaker.Unlock()
	player.dbg("playback started")
	// deadlocks if anything speaker related is done inside callback
	// since it's locking device itself
	// speaker.Play(beep.Seq(player.stream.volume, beep.Callback(
	//	func() {
	//	})))
	speaker.Play(player.stream.volume)
	return nil
}

func (player *beepPlayer) restart() {
	player.dbg("restart playback")
	speaker.Clear()
	//speaker.Play(beep.Seq(player.stream.volume, beep.Callback(
	//	func() {
	//	})))
	speaker.Play(player.stream.volume)
	player.status = playing
}

// stop is actually pause with position reset
func (player *beepPlayer) stop() {
	player.dbg("playback stopped")
	player.status = stopped
	if player.isReady() {
		player.resetPosition()
		speaker.Lock()
		player.stream.ctrl.Paused = true
		speaker.Unlock()
	}
}

// FIXME: doesn't return error, wait, it can't even return error
// no matter what is happening, since Close always returns nil
func (player *beepPlayer) clearStream() {
	player.dbg("clearing buffer")
	speaker.Clear()
	if player.isReady() {
		speaker.Lock()
		err := player.stream.streamer.Close()
		player.stream = nil
		speaker.Unlock()
		if err != nil {
			player.dbg(err.Error())
		}
	}
}

func (player *beepPlayer) playPause() bool {
	if !player.isReady() {
		return false
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
