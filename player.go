package main

import (
	"bytes"
	"fmt"
	"time"

	"github.com/faiface/beep"
	"github.com/faiface/beep/mp3"
	"github.com/faiface/beep/speaker"
)

var defaultSampleRate beep.SampleRate = 48000

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
	stream *mediaStream
	format beep.Format

	status         playbackStatus
	bufferedStatus playbackStatus
	volume         float64
	muted          bool

	// for debug reporting
	dbg func(string)
}

func newBeepPlayer(dbg func(string)) *beepPlayer {
	initializeDevice()
	return &beepPlayer{dbg: dbg}
}

// device initialization
func initializeDevice() {
	// TODO: add sample rate setting
	sr := beep.SampleRate(defaultSampleRate)
	speaker.Init(sr, sr.N(time.Second/10))
}

// play/pause/seekFWD/seekBWD count as active state
func (player *beepPlayer) isPlaying() bool {
	return player.status == playing || player.status == paused
}

func (player *beepPlayer) isReady() bool {
	return player.stream != nil
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

func (player *beepPlayer) seekRelative(offset int) error {
	if !player.isPlaying() {
		return nil
	}

	pos := player.format.SampleRate.N(
		time.Duration(offset) * time.Second)

	if offset > 0 {
		player.bufferedStatus = seekFWD
	} else {
		player.bufferedStatus = seekBWD
	}

	speaker.Lock()
	newPos := player.stream.streamer.Position() + pos

	if newPos < 0 {
		newPos = 0
	}

	if newPos >= player.stream.streamer.Len() {
		newPos = player.stream.streamer.Len() - 1
	}

	err := player.stream.streamer.Seek(newPos)
	speaker.Unlock()
	return err
}

func (player *beepPlayer) seekAbsolute(pos float64) error {
	if !player.isPlaying() {
		return nil
	}

	speaker.Lock()
	newPos := int(float64(player.stream.streamer.Len()) * pos)
	if newPos < 0 {
		newPos = 0
	}

	if newPos >= player.stream.streamer.Len() {
		newPos = player.stream.streamer.Len() - 1
	}

	err := player.stream.streamer.Seek(newPos)
	speaker.Unlock()
	return err
}

func (player *beepPlayer) play(data []byte) error {
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

func (player *beepPlayer) pause() {
	if !player.isReady() {
		return
	}

	player.status = paused
	speaker.Lock()
	player.stream.ctrl.Paused = true
	speaker.Unlock()
}

func (player *beepPlayer) playPause() {
	if !player.isReady() {
		return
	}

	switch player.status {

	case paused, stopped:
		player.status = playing

	case playing:
		player.status = paused
	}

	speaker.Lock()
	player.stream.ctrl.Paused = !player.stream.ctrl.Paused
	speaker.Unlock()
}

// stop is actually pause with position reset
func (player *beepPlayer) stop() {
	player.dbg("playback stopped")
	player.status = stopped
	if player.isReady() {
		player.seekAbsolute(0)
		speaker.Lock()
		player.stream.ctrl.Paused = true
		speaker.Unlock()
	}
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

func (player *beepPlayer) clearStream() {
	player.dbg("clearing buffer")
	speaker.Clear()
}

func (player *beepPlayer) getVolume() string {
	if player.muted {
		return "mute"
	} else {
		return fmt.Sprintf("%4.0f", (100 + player.volume*10))
	}
}

func (player *beepPlayer) getStatus() playbackStatus {
	if player.bufferedStatus < 0 {
		return player.status
	}
	status := player.bufferedStatus
	player.bufferedStatus = -1
	return status
}

func (player *beepPlayer) getTime() time.Duration {
	if player.isReady() {
		speaker.Lock()
		position := player.format.SampleRate.D(player.stream.streamer.Position())
		speaker.Unlock()
		return position
	}
	return 0
}

func (player *beepPlayer) getPosition() float64 {
	if player.isReady() {
		speaker.Lock()
		position := player.stream.streamer.Position()
		length := player.stream.streamer.Len()
		speaker.Unlock()
		return float64(position) / float64(length)
	}
	return 0
}
