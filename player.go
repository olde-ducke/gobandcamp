package main

import (
	"time"

	"github.com/faiface/beep"
)

// DefaultSampleRate sample rate that will be used.
var DefaultSampleRate beep.SampleRate = 48000

// Quality of resampling, for beep: 1-2 low, 3-4 medium,
// 5-6 high, higher values are not recommended.
var Quality = 1

// Statuses list of text representation of player current
// status: stopped, playing, paused, seeking backward/forward,
// skipping backward/forward (□ ▹ ▯▯ ◃◃ ▹▹ ▯◃ ▹▯).
var Statuses = [7]string{" \u25a1", " \u25b9", "\u25af\u25af",
	"\u25c3\u25c3", "\u25b9\u25b9", "\u25af\u25c3",
	"\u25b9\u25af"}

// PlaybackStatus player current state
type PlaybackStatus int

const (
	stopped PlaybackStatus = iota
	playing
	paused
	seekBWD
	seekFWD
	skipBWD
	skipFWD
)

func (status PlaybackStatus) String() string {
	return Statuses[status]
}

// Player is simple music player.
type Player interface {
	RaiseVolume()
	LowerVolume()
	Mute()
	SeekRelative(int) error
	SeekAbsolute(float64) error
	Load([]byte) error
	Reload() error
	Pause()
	Play()
	Stop()
	SetCallback(func())
	GetVolume() string
	GetStatus() PlaybackStatus
	SetStatus(PlaybackStatus)
	GetTime() time.Duration
	GetPosition() float64
	ClearStream()
}
