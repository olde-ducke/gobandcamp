package player

import (
	"bytes"
	"errors"
	"fmt"
	"time"

	"github.com/faiface/beep"
	"github.com/faiface/beep/mp3"
	"github.com/faiface/beep/speaker"
)

type beepPlayer struct {
	name string

	stream *mediaStream
	format beep.Format

	status         PlaybackStatus
	bufferedStatus PlaybackStatus
	volume         float64
	muted          bool

	// for debug reporting
	callback func()
}

// device initialization
func (player *beepPlayer) Init() error {
	// TODO: add sample rate setting

	sr := beep.SampleRate(DefaultSampleRate)
	return speaker.Init(sr, sr.N(time.Second/10))
}

// play/pause/seekFWD/seekBWD count as active state
func (player *beepPlayer) isPlaying() bool {
	return player.status == playing || player.status == paused
}

func (player *beepPlayer) isReady() bool {
	return player.stream != nil
}

func (player *beepPlayer) RaiseVolume() {
	player.volume += 0.5

	if player.volume > 0.0 {
		player.volume = 0.0
	}

	player.muted = false

	player.setVolume()
}

func (player *beepPlayer) LowerVolume() {
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

func (player *beepPlayer) Mute() {
	player.muted = !player.muted
	if player.isReady() {
		speaker.Lock()
		player.stream.volume.Silent = player.muted
		speaker.Unlock()
	}
}

func (player *beepPlayer) SeekRelative(offset int) error {
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
	err := player.setPosition(newPos)
	speaker.Unlock()
	return err
}

func (player *beepPlayer) SeekAbsolute(pos float64) error {
	if !player.isPlaying() {
		return nil
	}

	speaker.Lock()
	newPos := int(float64(player.stream.streamer.Len()) * pos)
	err := player.setPosition(newPos)
	speaker.Unlock()
	return err
}

// must be called under lock
func (player *beepPlayer) setPosition(pos int) error {
	if pos < 0 {
		pos = 0
	}

	if pos >= player.stream.streamer.Len() {
		pos = player.stream.streamer.Len() - 1
	}

	return player.stream.streamer.Seek(pos)
}

func (player *beepPlayer) Load(data []byte) error {
	reader := bytes.NewReader(data)
	streamer, format, err := mp3.Decode(NopSeekCloser(reader))
	if err != nil {
		return err
	}
	player.ClearStream()
	speaker.Lock()
	player.format = format
	player.stream = newStream(format.SampleRate, streamer, player.volume, player.muted)
	speaker.Unlock()
	Debugf(fmt.Sprintf("stream loaded: %+v", player.format))
	// deadlocks if anything speaker related is done inside callback
	// since it's locking device itself
	speaker.Play(beep.Seq(player.stream.volume, beep.Callback(
		func() {
			Debugf("callback")
			go player.callback()
			Debugf("callback exit")
		})))
	player.status = playing
	return player.stream.volume.Err()
}

func (player *beepPlayer) Reload() error {
	if !player.isReady() {
		return errors.New("nothing to reload")
	}
	Debugf("reload same stream")
	speaker.Clear()
	speaker.Lock()
	player.stream.ctrl.Paused = false
	speaker.Unlock()
	speaker.Play(beep.Seq(player.stream.volume, beep.Callback(
		func() {
			Debugf("callback")
			go player.callback()
			Debugf("callback exit")
		})))
	player.status = playing
	return player.stream.volume.Err()
}

func (player *beepPlayer) Pause() {
	if !player.isReady() || player.status != playing {
		Debugf("can't pause")
		return
	}

	Debugf("playback paused")
	player.status = paused
	speaker.Lock()
	player.stream.ctrl.Paused = true
	speaker.Unlock()
}

func (player *beepPlayer) Play() {
	if !player.isReady() {
		Debugf("can't play player isn't ready")
		return
	}

	switch player.status {

	case paused, stopped:
		player.status = playing

	case playing:
		player.status = paused

	default:
		Debugf("can't play while switching tracks")
		return
	}

	speaker.Lock()
	player.stream.ctrl.Paused = !player.stream.ctrl.Paused
	speaker.Unlock()
}

// Stop is actually pause with position reset
func (player *beepPlayer) Stop() {
	if !player.isReady() || !player.isPlaying() {
		Debugf("can't stop")
		return
	}

	err := player.SeekAbsolute(0)
	if err != nil {
		Debugf(err.Error())
		return
	}
	Debugf("playback stopped")
	player.status = stopped
	speaker.Lock()
	player.stream.ctrl.Paused = true
	speaker.Unlock()
}

// SetCallback sets function that will be called at the end of stream.
func (player *beepPlayer) SetCallback(f func()) {
	player.callback = f
}

func (player *beepPlayer) GetVolume() string {
	if player.muted {
		return "mute"
	}

	return fmt.Sprintf("%4.0f", (100 + player.volume*10))
}

func (player *beepPlayer) GetStatus() PlaybackStatus {
	if player.bufferedStatus >= stopped && player.bufferedStatus <= paused {
		return player.status
	}
	status := player.bufferedStatus
	player.bufferedStatus = player.status
	return status
}

// SetStatus accepts only skipFWD and skipBWD, other values discarded
func (player *beepPlayer) SetStatus(status PlaybackStatus) {
	if status != skipFWD && status != skipBWD {
		return
	}
	player.status = status
	player.bufferedStatus = status
}

func (player *beepPlayer) GetTime() time.Duration {
	if !player.isReady() {
		return 0
	}
	speaker.Lock()
	position := player.format.SampleRate.D(player.stream.streamer.Position())
	speaker.Unlock()
	return position
}

func (player *beepPlayer) GetPosition() float64 {
	if !player.isReady() {
		return 0
	}
	speaker.Lock()
	position := player.stream.streamer.Position()
	length := player.stream.streamer.Len()
	speaker.Unlock()
	return float64(position) / float64(length)
}

func (player *beepPlayer) ClearStream() {
	Debugf("clearing buffer")
	speaker.Clear()
	if player.isReady() {
		speaker.Lock()
		err := player.stream.streamer.Close()
		if err != nil {
			Debugf(err.Error())
		}
		player.stream = nil
		speaker.Unlock()
	}
}

func (player *beepPlayer) GetName() string {
	return player.name
}

func init() {
	p := &beepPlayer{name: "beep"}
	backends[p.GetName()] = p
}
