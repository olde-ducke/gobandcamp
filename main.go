package main

import (
	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/faiface/beep"
	"github.com/faiface/beep/mp3"
	"github.com/faiface/beep/speaker"
)

// TODO: cache cleaning
var cache map[int][]byte
var player playback
var logFile *os.File

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
	//albumList    *Album // not a list at the moment
	totalTracks int
	stream      *mediaStream
	format      beep.Format

	status         playbackStatus
	bufferedStatus playbackStatus
	playbackMode   playbackMode
	volume         float64
	muted          bool
}

func (player *playback) changePosition(pos int) time.Duration {
	newPos := player.stream.streamer.Position()
	newPos += pos
	if newPos < 0 {
		newPos = 0
	}
	if newPos >= player.stream.streamer.Len() {
		newPos = player.stream.streamer.Len() - 1
	}
	if err := player.stream.streamer.Seek(newPos); err != nil {
		// FIXME:
		// sometimes reports errors, for example this one:
		// https://github.com/faiface/beep/issues/116
		// causes track to skip, again, only sometimes
		window.sendPlayerEvent(err)
	}
	return player.format.SampleRate.D(newPos).Round(time.Second)
}

func (player *playback) resetPosition() {
	if err := player.stream.streamer.Seek(0); err != nil {
		window.sendPlayerEvent(err)
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
	if player.playbackMode == random {
		player.nextTrack()
		return
	}
	//player.stop()
	if player.totalTracks == 0 {
		return
	}
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
	// FIXME: random playback skips this
	// to prevent downloading every item on fast track switching
	// probably dumb idea, -race yells with warnings about it
	go func() {
		if _, ok := cache[player.currentTrack]; ok {
			window.sendPlayerEvent(eventNextTrack(player.currentTrack))
		} else {
			track := player.currentTrack
			time.Sleep(time.Second / 3)
			if track == player.currentTrack {
				window.sendPlayerEvent(eventNextTrack(player.currentTrack))
			}
		}
	}()
}

func (player *playback) nextTrack() {
	switch player.playbackMode {
	case random:
		rand.Seed(time.Now().UnixNano())
		player.currentTrack = rand.Intn(player.totalTracks)
		window.sendPlayerEvent(eventNextTrack(player.currentTrack))
	case repeatOne:
		window.sendPlayerEvent(eventNextTrack(player.currentTrack))
	case repeat:
		player.skip(true)
	case normal:
		if player.currentTrack == player.totalTracks-1 {
			// can't do anything with speaker when called from callback
			// added stop to play function, that actually
			// prevents double playback
			player.status = stopped
			return
		}
		player.skip(true)
	}
	player.status = skipFWD
}

func (player *playback) play(track int) {
	// TODO: update current track for playlist view
	player.stop()
	streamer, format, err := mp3.Decode(wrapInRSC(track))
	if err != nil {
		window.sendPlayerEvent(err)
		return
	}
	speaker.Lock()
	player.format = format
	player.stream = newStream(format.SampleRate, streamer, player.volume, player.muted)
	player.status = playing
	speaker.Unlock()
	//speaker.Play(player.stream.volume)
	speaker.Play(beep.Seq(player.stream.volume, beep.Callback(
		player.nextTrack)))
}

func (player *playback) stop() {
	if player.isReady() {
		speaker.Clear()
		speaker.Lock()
		player.resetPosition()
		err := player.stream.streamer.Close()
		if err != nil {
			window.sendPlayerEvent(err)
		}
		speaker.Unlock()
	}
	player.status = stopped
}

func (player *playback) initPlayer() {
	cache = make(map[int][]byte)
	// keeps volume and playback mode from previous playback
	*player = playback{playbackMode: player.playbackMode, volume: player.volume,
		muted: player.muted}
}

func checkFatalError(err error) {
	if err != nil {
		app.Quit()
		logFile.WriteString(time.Now().Format(time.ANSIC) + "[err]:" + err.Error())
		// FIXME: can't print while app is finishing
		fmt.Fprintln(os.Stderr, err)
		exitCode = 1
	}
}

// device initialization
func init() {
	var err error
	sr := beep.SampleRate(defaultSampleRate)
	speaker.Init(sr, sr.N(time.Second/10))
	logFile, err = os.Create("dump.log")
	checkFatalError(err)
}

func (player *playback) handleEvent(key rune) bool {
	switch key {
	// TODO: change controls
	case ' ':
		// FIXME ???
		if player.status == seekBWD || player.status == seekFWD {
			player.status = player.bufferedStatus
		}
		if !player.isReady() {
			return false
		}
		if player.status == playing {
			player.status = paused
		} else if player.status == paused {
			player.status = playing
		} else if player.status == stopped {
			player.play(player.currentTrack)
			return true
		}
		speaker.Lock()
		player.stream.ctrl.Paused = !player.stream.ctrl.Paused
		speaker.Unlock()

	case 'a', 'A':
		if !player.isPlaying() {
			return false
		} else if player.status != seekBWD && player.status != seekFWD {
			player.bufferedStatus = player.status
			player.status = seekBWD
		}
		speaker.Lock()
		player.changePosition(0 - player.format.SampleRate.N(time.Second*3))
		speaker.Unlock()

	case 'd', 'D':
		if !player.isPlaying() {
			return false
		} else if player.status != seekFWD && player.status != seekBWD {
			player.bufferedStatus = player.status
			player.status = seekFWD
		}
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
		// FIXME: something weird is going on here
		if player.getCurrentTrackPosition().Round(time.Second) > time.Second*3 &&
			player.isReady() {

			speaker.Lock()
			player.resetPosition()
			speaker.Unlock()
		} else {
			player.skip(false)
		}

	case 'f', 'F':
		player.skip(true)

	case 'p', 'P':
		if player.isReady() {
			player.stop()
		}
	default:
		return false
	}
	window.sendPlayerEvent(nil)
	return true
}

func main() {
	//
	// FIXME: behaves weird after coming from suspend (high CPU load)
	// FIXME: device does not reinitialize after suspend
	// FIXME: takes device to itself, doesn't allow any other program to use it, and can't use it, if device is already being used
	//player.initPlayer()
	// just switch to SDL, it doesn't have any of these problems
	// FIXME: can't tell orientation on the start for whatever reason
	err := app.Run()
	checkFatalError(err)
	logFile.Close()
	os.Exit(exitCode)
}
