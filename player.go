package main

import (
	"bytes"
	"fmt"
	"math"
	"math/rand"
	"time"

	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/audio/mp3"
)

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

// FIXME: this does not belong here
// □ ▹ ▯▯ ◃◃ ▹▹ ▯◃ ▹▯
// fancy stuff, but doesn't work everywhere
func (status playbackStatus) String() string {
	return [7]string{" \u25a1", " \u25b9", "\u25af\u25af",
		"\u25c3\u25c3", "\u25b9\u25b9", "\u25af\u25c3",
		"\u25b9\u25af"}[status]
}

type streamPlayer struct {
	currentTrack int
	totalTracks  int
	p            *audio.Player
	ctx          *audio.Context
	timeStep     time.Duration
	duration     time.Duration
	sampleRate   int

	status         playbackStatus
	bufferedStatus playbackStatus
	playbackMode   playbackMode
	volume         float64
	muted          bool

	text chan<- interface{}
	next chan<- struct{}
}

func newPlayer(sampleRate int, text chan<- interface{}, next chan<- struct{}) *streamPlayer {
	ctx := audio.NewContext(sampleRate)
	return &streamPlayer{
		ctx:        ctx,
		timeStep:   2 * time.Second,
		sampleRate: sampleRate,
		volume:     1.0,
		text:       text,
		next:       next,
	}
}

func (p *streamPlayer) raiseVolume() {
	p.volume += 0.05

	if p.volume > 1.0 {
		p.volume = 1.0
	}

	p.muted = false

	p.setVolume()
}

func (p *streamPlayer) lowerVolume() {
	p.volume -= 0.05
	if p.volume < 0.0 {
		p.volume = 0.0
	}

	if p.volume < 0.01 {
		p.muted = true
	}

	p.setVolume()
}

func (p *streamPlayer) setVolume() {
	if p.p != nil {
		p.p.SetVolume(p.adjustedVolume())
	}
}

func (p *streamPlayer) mute() {
	p.muted = !p.muted
	if p.p != nil {
		if p.muted {
			p.p.SetVolume(0.0)
			return
		}

		p.p.SetVolume(p.adjustedVolume())
	}
}

func (p *streamPlayer) getVolume() string {
	if p.muted {
		return "mute"
	}
	return fmt.Sprintf("%4.0f", p.volume*100)
}

func (p *streamPlayer) getCurrentTrack() int {
	return p.currentTrack
}

func (p *streamPlayer) getPlaybackMode() string {
	return p.playbackMode.String()
}

func (p *streamPlayer) getStatus() playbackStatus {
	if p.bufferedStatus < 0 {
		return p.status
	}
	status := p.bufferedStatus
	p.bufferedStatus = -1
	return status
}

func (p *streamPlayer) seek(forward bool) bool {
	if !p.isPlaying() {
		return false
	}

	if p.p == nil {
		return false
	}

	pos := p.p.Position()

	offset := p.timeStep
	if forward {
		p.bufferedStatus = seekFWD
	} else {
		p.bufferedStatus = seekBWD
		offset *= -1
	}

	pos += offset
	if pos < 0 {
		pos = 0
	} else if pos > p.duration {
		pos = p.duration
	}

	if err := p.p.SetPosition(pos); err != nil {
		if err != nil {
			p.text <- err
		}
	}

	return true
}

func (p *streamPlayer) resetPosition() {
	p.text <- "reset position"
	if err := p.p.Rewind(); err != nil {
		p.text <- err
	}
}

func (p *streamPlayer) getCurrentTrackPosition() time.Duration {
	if p.p != nil {
		return p.p.Position().Truncate(time.Second)
	}
	return 0
}

// play/pause/seekFWD/seekBWD count as active state
func (p *streamPlayer) isPlaying() bool {
	return p.status == playing || p.status == paused
}

func (p *streamPlayer) skip(direction int) bool {
	if p.totalTracks == 0 {
		return false
	}

	if p.playbackMode == random {
		p.nextTrack()
		return true
	}

	p.text <- "skip track"
	p.stop()
	p.clearStream()

	switch {
	case direction > 0:
		p.currentTrack = (p.currentTrack + 1) %
			p.totalTracks
		fallthrough
	case direction == 0:
		p.status = skipFWD
	case direction < 0:
		p.currentTrack = (p.totalTracks +
			p.currentTrack - 1) %
			p.totalTracks
		p.status = skipBWD
	}

	// TODO: remove after response reading cancellation is implemented
	go p.delaySwitching()
	return true
}

func (p *streamPlayer) nextMode() {
	p.playbackMode = (p.playbackMode + 1) % 4
}

func (p *streamPlayer) nextTrack() {
	p.text <- "next track"
	switch p.playbackMode {

	case random:
		var previousTrack int

		if p.totalTracks > 1 {
			previousTrack = p.currentTrack
			// never play same track again if random
			for p.currentTrack == previousTrack {
				p.currentTrack = rand.Intn(p.totalTracks)
			}
		}
		p.stop()

		if p.currentTrack >= previousTrack {
			p.status = skipFWD
		} else {
			p.status = skipBWD
		}

		p.clearStream()
		go p.delaySwitching()

	case repeatOne:
		p.skip(0)

	case repeat:
		p.skip(1)

	case normal:
		if p.currentTrack == p.totalTracks-1 {
			p.stop()
			return
		}
		p.skip(1)
	}
}

// to prevent downloading every item on fast track switching
// probably dumb idea
func (p *streamPlayer) delaySwitching() {
	track := p.currentTrack

	time.Sleep(time.Second / 2)

	if track == p.currentTrack {
		window.sendEvent(newTrack(p.currentTrack))
	}
}

func (p *streamPlayer) setTrack(track int) {
	p.stop()
	p.clearStream()
	if track >= p.currentTrack {
		p.status = skipFWD
	} else {
		p.status = skipBWD
	}
	p.currentTrack = track
	go p.delaySwitching()
}

func (p *streamPlayer) play(key string) {
	// FIXME: this code does not belong here
	value, ok := cache.get(key)
	if !ok {
		p.text <- "missing cache entry"
		return
	}

	// FIXME: there are a lot of mental acrobatics below just
	//        to get real duration out of the stream and not
	//        from server response, this is needed to not get
	//        out of bounds when setting postition, which is
	//        for some reason is not done automagically
	s, err := mp3.DecodeF32(bytes.NewReader(value))
	if err != nil {
		p.text <- err
		return
	}

	// this function returns interface, which does not
	// implement Length() int64, why?
	r := audio.ResampleReaderF32(s, s.Length(), s.SampleRate(), p.sampleRate)

	stream := newCallback(r, func() { p.next <- struct{}{} })

	if p.p != nil {
		p.p.Close()
	}

	size := r.(length).Length()

	p.duration = time.Duration(size/(2*int64(p.sampleRate)*4)) * time.Second

	p.p, err = p.ctx.NewPlayerF32(stream)
	if err != nil {
		p.text <- err
		return
	}

	p.status = playing

	if p.muted {
		p.p.SetVolume(0.0)
	} else {
		p.p.SetVolume(p.adjustedVolume())
	}

	p.text <- "playback started"
	p.p.Play()
}

func (p *streamPlayer) restart() {
	p.text <- "restart playback"
	if p.p != nil {
		p.resetPosition()
		p.p.Play()
		p.status = playing
	}
}

// stop is actually pause with position reset
func (p *streamPlayer) stop() {
	p.text <- "playback stopped"
	if p.p != nil {
		p.resetPosition()
		p.p.Pause()
		p.status = stopped
	}
}

func (p *streamPlayer) clearStream() {
	p.text <- "clearing buffer"
	if p.p != nil {
		err := p.p.Close()
		p.p = nil
		p.duration = 0
		if err != nil {
			p.text <- err
		}
	}
}

func (p *streamPlayer) playPause() bool {
	if p.p == nil {
		return false
	}

	if p.status == seekBWD || p.status == seekFWD {
		p.status = p.bufferedStatus
	}

	switch p.status {

	case paused, stopped:
		p.status = playing
		p.p.Play()

	case playing:
		p.status = paused
		p.p.Pause()

	default:
		return false
	}

	return true
}

func (p *streamPlayer) Close() error {
	if p.p != nil {
		return p.p.Close()
	}

	return nil
}

func (p *streamPlayer) adjustedVolume() float64 {
	// https://forum.gamemaker.io/index.php?threads/master-sound-volume-conversion-logarithmic-linear.85575/
	return math.Pow(10, p.volume*3-1) / 100
}
