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
	if player.isReady() {
		window.sendPlayerEvent(eventDebugMessage("reset position"))
		if err := player.stream.streamer.Seek(0); err != nil {
			window.sendPlayerEvent(err)
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
	defer player.stop()
	window.sendPlayerEvent(eventDebugMessage("skip track"))
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
	//window.sendPlayerEvent(eventNextTrack(player.currentTrack))
	go func() {
		//cache.mu.Lock()
		//defer cache.mu.Unlock()
		if _, ok := cache.bytes[player.currentTrack]; ok {
			window.sendPlayerEvent(eventNextTrack(player.currentTrack))
		} else {
			track := player.currentTrack
			time.Sleep(time.Second / 2)
			if track == player.currentTrack {
				window.sendPlayerEvent(eventNextTrack(player.currentTrack))
			}
		}
	}()
}

func (player *playback) nextTrack() {
	defer player.stop()
	window.sendPlayerEvent(eventDebugMessage("next track"))
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
	// FIXME: it is possible to play two first tracks at the same time, if you input
	// two queries fast enough, this stops previous playback
	player.stop()
	window.sendPlayerEvent(eventDebugMessage("music play"))
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
	// deadlocks if anything speaker related is done inside callback
	// since it's locking device itself
	speaker.Play(beep.Seq(player.stream.volume, beep.Callback(
		func() { go player.nextTrack() })))
}

func (player *playback) stop() {
	window.sendPlayerEvent(eventDebugMessage("music stopped"))
	speaker.Clear()
	if player.isReady() {
		speaker.Lock()
		player.resetPosition()
		// FIXME: doesn't really matter if we close streamer or not
		// method close in underlying data does absolutely nothing
		err := player.stream.streamer.Close()
		speaker.Unlock()
		if err != nil {
			window.sendPlayerEvent(err)
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
	//cache.mu.Lock()
	// main loop might end up in dedlock if locked here
	window.sendPlayerEvent(eventDebugMessage("player reset"))
	cache.bytes = make(map[int][]byte)
	// keeps volume and playback mode from previous playback
	*player = playback{playbackMode: player.playbackMode, volume: player.volume,
		muted: player.muted}
	//cache.mu.Unlock()
}

func (player *playback) handleEvent(key rune) bool {
	switch key {
	// TODO: change controls
	case ' ':
		// FIXME ???
		if player.status == seekBWD || player.status == seekFWD {
			player.status = player.bufferedStatus
		}
		if player.status == stopped {
			player.play(player.currentTrack)
			return true
		}
		if !player.isPlaying() {
			return false
		}
		if player.status == playing {
			player.status = paused
		} else if player.status == paused {
			player.status = playing
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
	window.sendPlayerEvent(nil)
	return true
}

// device initialization
func init() {
	sr := beep.SampleRate(defaultSampleRate)
	speaker.Init(sr, sr.N(time.Second/10))
}
