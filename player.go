package main

import "time"

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

type Player interface {
	RaiseVolume()
	LowerVolume()
	Mute()
	SeekRelative(int) error
	SeekAbsolute(float64) error
	Load([]byte) error
	Pause()
	PlayPause()
	Stop()
	GetVolume() string
	GetStatus() playbackStatus
	GetTime() time.Duration
	GetPosition() float64
	ClearStream()
}
