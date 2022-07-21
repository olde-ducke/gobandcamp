package player

import (
	"fmt"
	"sync"
	"time"
)

type dummyPlayer struct {
	sync.Mutex

	status         PlaybackStatus
	bufferedStatus PlaybackStatus
	volume         float64
	muted          bool
	name           string
	ready          bool

	callback func()
}

func (player *dummyPlayer) isPlaying() bool {
	return player.status == playing || player.status == paused
}

func (player *dummyPlayer) Init() error {
	return nil
}

func (player *dummyPlayer) RaiseVolume() {
	player.volume += 5
	if player.volume > 100.0 {
		player.volume = 100.0
	}
	player.muted = false
}

func (player *dummyPlayer) LowerVolume() {
	player.volume -= 5
	if player.volume <= 0.0 {
		player.volume = 0.0
		player.muted = true
	}
}

func (player *dummyPlayer) Mute() {
	player.muted = !player.muted
}

func (player *dummyPlayer) SeekRelative(int) error {
	if player.isPlaying() {

	}

	return nil
}

func (player *dummyPlayer) SeekAbsolute(float64) error {
	return nil
}

func (player *dummyPlayer) Load(m *Media) error {
	if m == nil {
		return ErrEmptyData
	}

	Debugf("got data with length: %d, content-type: %s", len(m.Data), m.ContentType)
	player.ClearStream()
	return player.Reload()
}

func (player *dummyPlayer) Reload() error {
	player.Lock()
	player.status = playing
	player.ready = true
	player.Unlock()
	return nil
}

func (player *dummyPlayer) Pause() {
	if player.status != playing {
		return
	}
	player.Lock()
	player.status = paused
	player.Unlock()
}

func (player *dummyPlayer) Play() {
	if !player.ready {
		return
	}

	var status PlaybackStatus
	switch player.status {
	case paused, stopped:
		status = playing
	case playing:
		status = paused
	default:
		return
	}

	player.Lock()
	player.status = status
	player.Unlock()
}

func (player *dummyPlayer) Stop() {
	if !player.isPlaying() {
		return
	}

	player.Lock()
	player.status = stopped
	player.Unlock()
}

func (player *dummyPlayer) SetCallback(f func()) {
	player.callback = f
}

func (player *dummyPlayer) GetVolume() string {
	if player.muted {
		return "mute"
	}

	return fmt.Sprintf("%4.0f", player.volume)
}

func (player *dummyPlayer) GetStatus() PlaybackStatus {
	if player.bufferedStatus >= stopped &&
		player.bufferedStatus <= paused {
		return player.status
	}

	status := player.bufferedStatus
	player.bufferedStatus = player.status
	return status
}

func (player *dummyPlayer) SetStatus(status PlaybackStatus) {
	if status != skipFWD && status != skipBWD {
		return
	}
	player.status = status
	player.bufferedStatus = status
}

func (player *dummyPlayer) GetTime() time.Duration {
	return time.Second * 5
}

func (player *dummyPlayer) GetPosition() float64 {
	return 0.0
}

func (player *dummyPlayer) ClearStream() {
	player.Lock()
	player.ready = false
	player.Unlock()
}

func (player *dummyPlayer) GetName() string {
	return player.name
}

func init() {
	p := &dummyPlayer{name: "dummy"}
	backends[p.GetName()] = p
}
