package main

import (
	"bufio"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/faiface/beep"
	"github.com/faiface/beep/mp3"
	"github.com/faiface/beep/speaker"
	"github.com/gdamore/tcell/v2"
)

// TODO: cache cleaning
var cachedResponses map[int][]byte

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
	albumList    *Album // not a list at the moment
	stream       *mediaStream
	format       beep.Format

	status        playbackStatus
	playbackMode  playbackMode
	currentPos    time.Duration
	latestMessage string
	volume        float64
	muted         bool

	event chan interface{}
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
		player.latestMessage = fmt.Sprint("Seek: ", err.Error())
	}
	return player.format.SampleRate.D(newPos).Round(time.Second)
}

func (player *playback) resetPosition() {
	if err := player.stream.streamer.Seek(0); err != nil {
		player.latestMessage = err.Error()
	}
}

func (player *playback) getCurrentTrackPosition() time.Duration {
	return player.format.SampleRate.D(player.stream.
		streamer.Position()).Round(time.Second)
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
	player.stop()
	if forward {
		player.currentTrack = (player.currentTrack + 1) %
			player.albumList.Tracks.NumberOfItems
		player.status = skipFWD
	} else {
		player.currentTrack = (player.albumList.Tracks.NumberOfItems +
			player.currentTrack - 1) %
			player.albumList.Tracks.NumberOfItems
		player.status = skipBWD
	}
	// to prevent downloading every item on fast track switching
	// probably dumb idea, -race yells with warnings about it
	go func() {
		if _, ok := cachedResponses[player.currentTrack]; ok {
			player.getNewTrack(player.currentTrack)
		} else {
			track := player.currentTrack
			time.Sleep(time.Second / 2)
			if track == player.currentTrack {
				player.getNewTrack(player.currentTrack)
			}
		}
	}()
}

func (player *playback) nextTrack() {
	player.stop()
	switch player.playbackMode {
	case random:
		rand.Seed(time.Now().UnixNano())
		player.currentTrack = rand.Intn(player.albumList.Tracks.NumberOfItems)
		go player.getNewTrack(player.currentTrack)
	case repeatOne:
		go player.getNewTrack(player.currentTrack)
	case repeat:
		player.skip(true)
	case normal:
		if player.currentTrack == player.albumList.Tracks.NumberOfItems-1 {
			return
		}
		player.skip(true)
	}
	player.status = skipFWD
}

func (player *playback) play(track int) error {
	streamer, format, err := mp3.Decode(wrapInRSC(track))
	if err != nil {
		player.status = stopped
		return err
	}
	speaker.Lock()
	player.format = format
	player.stream = newStream(format.SampleRate, streamer, player.volume, player.muted)
	player.status = playing
	speaker.Unlock()
	speaker.Play(beep.Seq(player.stream.volume, beep.Callback(func() {
		player.event <- true
	})))
	return nil
}

func (player *playback) stop() {
	speaker.Clear()
	speaker.Lock()
	if player.isReady() {
		player.resetPosition()
		err := player.stream.streamer.Close()
		if err != nil {
			player.latestMessage = err.Error()
		}
	}
	speaker.Unlock()
	player.status = stopped
}

func (player *playback) initPlayer(album *Album) {
	cachedResponses = make(map[int][]byte)
	// keeps volume and playback mode from previous playback
	*player = playback{playbackMode: player.playbackMode, volume: player.volume,
		muted: player.muted, albumList: album}
	player.albumList.AlbumArt = getPlaceholderImage()
	player.event = make(chan interface{})
	go player.downloadCover()
	go player.getNewTrack(player.currentTrack)
}

// TODO: comand line arguments and input parser for tag search
func handleInput(message string) (jsonString string) {
	for {
		stdinReader := bufio.NewReader(os.Stdin)
		fmt.Println(message)
		input, err := stdinReader.ReadString('\n')
		reportError(err)
		switch input {
		case "\n":
			return ""
		case "exit\n", "q\n":
			return "q"
		default:
			jsonString, err = getAlbumPage(strings.Trim(input, "\n"))
		}
		if err == nil {
			break
		} else {
			fmt.Println("Error:", err)
		}
	}
	return jsonString
}

func reportError(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// device initialization
func init() {
	sr := beep.SampleRate(defaultSampleRate)
	speaker.Init(sr, sr.N(time.Second/10))
}

func main() {
	var player playback
	// FIXME: behaves weird after coming from suspend (high CPU load)
	// FIXME: device does not reinitialize after suspend
	// FIXME: takes device to itself, doesn't allow any other program to use it, and can't use it, if device is already being used
	var parsedString string
	for parsedString == "" {
		parsedString = handleInput("Enter bandcamp album link, type `exit` or q to quit")
		if parsedString == "q" {
			os.Exit(0)
		}
	}

	player.initPlayer(parseJSON(parsedString))

	drawer := screenDrawer{
		fgColor:  tcell.NewHexColor(0xf9fdff),
		bgColor:  tcell.NewHexColor(0x2b2b2b),
		altColor: tcell.NewHexColor(0x999999),
	}
	drawer.initScreen(&player)
	ticker := time.NewTicker(time.Second / 2)
	timer := ticker.C
	tcellEvent := make(chan tcell.Event)
	// FIXME: probably breaks something: store real player status
	// and display new status while key is held down, then update it
	// on next timer tick (tcell can't tell when key is released)
	var bufferedStatus playbackStatus

	// FIXME: can cause weird behaviour when going back from terminal
	// problem should go away if we implement input inside tcell itself
	go func() {
		for {
			tcellEvent <- drawer.screen.PollEvent()
		}
	}()

	// event loop
	for {
		select {
		case <-timer:
			if player.status == seekBWD || player.status == seekFWD {
				player.status = bufferedStatus
			}
			if player.isReady() {
				speaker.Lock()
				player.currentPos = player.getCurrentTrackPosition()
				speaker.Unlock()
				drawer.updateTextData(&player)
			}

		// TODO: this is kinda janky, implement something that makes sense
		// to handle player events
		case event := <-player.event:
			switch value := event.(type) {
			case bool:
				player.nextTrack()
				drawer.updateTextData(&player)
			case int:
				if event == player.currentTrack && player.status != playing {
					err := player.play(value)
					if err != nil {
						player.latestMessage = fmt.Sprint("Error creating new stream:", err)
					}
					drawer.updateTextData(&player)
				}
			case string:
				player.latestMessage = value
				drawer.reDrawMetaData(&player)
			case *tcell.EventResize:
				drawer.reDrawMetaData(&player)
			}

		// handle tcell events
		// TODO: change controls
		case event := <-tcellEvent:
			switch event := event.(type) {
			case *tcell.EventResize:
				drawer.reDrawMetaData(&player)
			case *tcell.EventKey:
				if event.Key() == tcell.KeyESC {
					drawer.screen.Fini()
					player.stop()
					// crashes after suspend
					// speaker.Close()
					ticker.Stop()
					os.Exit(0)
				}

				if event.Key() != tcell.KeyRune {
					continue
				}

				switch event.Rune() {
				case ' ':
					// FIXME ???
					if !player.isReady() {
						continue
					}
					if player.status == seekBWD || player.status == seekFWD {
						player.status = bufferedStatus
					}
					if player.status == playing {
						player.status = paused
					} else if player.status == paused {
						player.status = playing
					} else if player.status == stopped {
						err := player.play(player.currentTrack)
						if err != nil {
							player.latestMessage = err.Error()
						}
						continue
					} else {
						continue
					}
					speaker.Lock()
					player.stream.ctrl.Paused = !player.stream.ctrl.Paused
					speaker.Unlock()
				case 'a', 'A':
					if !player.isPlaying() {
						continue
					} else if player.status != seekBWD && player.status != seekFWD {
						bufferedStatus = player.status
						player.status = seekBWD
					}
					speaker.Lock()
					player.currentPos =
						player.changePosition(
							0 - player.format.SampleRate.N(time.Second*3))
					speaker.Unlock()

				case 'd', 'D':
					if !player.isPlaying() {
						continue
					} else if player.status != seekFWD && player.status != seekBWD {
						bufferedStatus = player.status
						player.status = seekFWD
					}
					speaker.Lock()
					player.currentPos =
						player.changePosition(
							player.format.SampleRate.N(time.Second * 3))
					speaker.Unlock()

				case 's', 'S':
					if !player.isReady() || player.volume < -9.6 {
						continue
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
						continue
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
						continue
					}
					player.muted = !player.muted
					speaker.Lock()
					player.stream.volume.Silent = player.muted
					speaker.Unlock()

				case 'r', 'R':
					player.playbackMode = (player.playbackMode + 1) % 4

				case 'b', 'B':
					// FIXME: something weird is going on here
					if player.currentPos.Round(time.Second) > time.Second*3 &&
						player.isReady() {

						speaker.Lock()
						player.resetPosition()
						speaker.Unlock()
					} else {
						player.skip(false)
					}

				case 'f', 'F':
					player.skip(true)

				case 't', 'T':
					drawer.lightMode = !drawer.lightMode
					fgColor, bgColor, _ := drawer.style.Decompose()
					drawer.style = drawer.style.Foreground(bgColor).
						Background(fgColor)
					drawer.reDrawMetaData(&player)

				case 'i', 'I':
					drawer.artMode = (drawer.artMode + 1) % 6
					drawer.redrawArt(&player)

				case 'p', 'P':
					if player.isReady() {
						player.stop()
					}
				// FIXME: hangs sometimes, not all deadlocks are resolved
				// probably poll events listener in other goroutine
				// trying to get events from screen that is disabled
				case 'o', 'O':
					drawer.screen.Fini()
					parsedString = handleInput("Enter new album link, leave empty to go back")
					if parsedString == "q" {
						player.stop()
						// crashes after suspend
						// speaker.Close()
						ticker.Stop()
						os.Exit(0)
					} else if parsedString != "" {
						player.stop()
						player.initPlayer(parseJSON(parsedString))
					}
					drawer.initScreen(&player)
					continue
				}
				drawer.updateTextData(&player)
			}
		}
		// TODO: remove later, sometimes hangs anyway without any reasonable
		// error
		// didn't hang for a while, somehow fixed???
		if player.isReady() {
			speaker.Lock()
			reportError(player.stream.streamer.Err())
			reportError(player.stream.ctrl.Err())
			reportError(player.stream.resampler.Err())
			reportError(player.stream.volume.Err())
			speaker.Unlock()
		}
	}
}
