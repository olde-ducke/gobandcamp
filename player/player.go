package player

import (
	"fmt"
	"strings"
	"time"

	"github.com/faiface/beep"
)

// DefaultSampleRate sample rate that will be used.
var DefaultSampleRate beep.SampleRate = 44100

// Quality of resampling, for beep: 1-2 low, 3-4 medium,
// 5-6 high, higher values are not recommended.
var Quality = 1

// Statuses list of text representation of player current
// status: stopped, playing, paused, seeking backward/forward,
// skipping backward/forward ([] > || << >> |< >|).
var Statuses = [7]string{"[]", " >", "||", "<<", ">>", "|<", ">|"}

// Debugf package level function for debug printing
// by default does nothing.
var Debugf = func(string, ...any) {}

var backends = make(map[string]Player, 3)

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
	if status < stopped || status > skipFWD {
		return "na"
	}

	return Statuses[status]
}

// Player is simple music player.
type Player interface {
	Init() error
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
	GetName() string
}

func NewPlayer(snd string) (Player, error) {
	player, ok := backends[snd]
	if !ok {
		return nil, fmt.Errorf("sound backend \"%s\" is not available", snd)
	}

	err := player.Init()
	if err != nil {
		return nil, err
	}

	Debugf(fmt.Sprintf("initializing %s player, sample rate: %d, resampling quality: %d",
		player.GetName(), DefaultSampleRate, Quality))
	return player, nil
}

func AvailableBackends() string {
	if len(backends) == 0 {
		return "none"
	}

	v := make([]string, 0, len(backends))
	for k := range backends {
		v = append(v, k)
	}

	return strings.Join(v, ", ")
}
