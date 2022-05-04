package main

import (
	"bufio"
	"fmt"
	"os"
	"sync"
)

var dummyData = item{
	artist:   "test artist",
	url:      "https://albumurl",
	tags:     []string{"test", "another tag", "test"},
	title:    "album title test",
	artURL:   "art url",
	hasAudio: true,
	tracks: []track{{
		streaming:       1,
		unreleasedTrack: false,
		mp3128:          "https://testpath",
		title:           "track title",
		artist:          "track artist",
		trackNumber:     26,
		url:             "https://trackurl",
		duration:        666.66,
	}}}

func getDecoration(t messageType) string {
	switch t {
	case debugMessage:
		return "\x1b[33m"
	case errorMessage:
		return "\x1b[31m"
	case textMessage:
		return "\x1b[36m"
	case infoMessage:
		return "\x1b[32m"
	default:
		return "\x1b[34m"
	}
}

type headless struct {
	wg           sync.WaitGroup
	active       bool
	formatString string
	prevMessage  *message
	player       Player
	playlist     *Playlist
	do           chan<- *action
}

func (h *headless) Run(quit chan<- struct{}, do chan<- *action) {
	h.do = do
	h.active = true
	h.wg.Add(1)
	go h.start()
	h.wg.Wait()
	h.displayInternal("goodbye")
	defer close(quit)
}

func (h *headless) Quit() {
	h.active = false
}

func (h *headless) Update() {
	if !h.active {
		return
	}
	title, album, artist := "---", "---", "---"
	var sep, trackArtist, duration string
	t := h.playlist.GetCurrentTrack()
	total := h.playlist.GetTotalTracks()
	// FIXME: will absolutely fail
	if !h.playlist.IsEmpty() {
		current := h.playlist.GetCurrentItem()
		title = current.Title
		album = current.Album
		artist = current.Artist
		if current.TrackArtist != "" {
			sep = "\x1b[0m - \x1b[35m"
			trackArtist = current.TrackArtist
		}
		if current.Duration > 0 {
			d := int(current.Duration)
			if d > 3600 {
				duration = fmt.Sprintf("%02d:%02d:%02d", d/3600%99, d/60%60, d%60)
			} else {
				duration = fmt.Sprintf("%02d:%02d", d/60%60, d%60)
			}
		}
	}
	pos := int(h.player.GetTime().Seconds())
	fmt.Printf("\x1b[s\x1b[F\x1b[0K%10s %02d:%02d:%02d \x1b[35m[plr]:\x1b[0m %2s \x1b[35m%s%s%s\x1b[0m from \x1b[35m%s\x1b[0m by \x1b[35m%s\x1b[0m %s\x1b[u",
		fmt.Sprintf("%d/%d", t, total), pos/3600%99, pos/60%60, pos%60,
		h.player.GetStatus(), trackArtist, sep, title, album, artist, duration)
}

func (h *headless) displayInternal(text string) {
	h.DisplayMessage(newMessage(infoMessage, "", "ui: "+text))
}

func (h *headless) DisplayMessage(msg *message) {
	defer h.Update()
	decoration := getDecoration(msg.msgType)
	if msg.When().Before(h.prevMessage.When()) {
		fmt.Printf(h.formatString, msg.When().Format("2006/01/02 15:04:05"),
			decoration, msg.msgType, "\x1b[31mMESSAGE IS LATE\x1b[0m "+msg.prefix, msg.text)
		return
	}

	h.prevMessage = msg
	fmt.Printf(h.formatString, msg.When().Format("2006/01/02 15:04:05"),
		decoration, msg.msgType, msg.prefix, msg.text)
}

func (h *headless) start() {
	fmt.Println()
	h.Update()

	scanner := bufio.NewScanner(os.Stdin)
	for h.active {
		scanner.Scan()
		input := scanner.Text()
		fmt.Print("\x1b[F\x1b[0K")
		switch input {
		case "q":
			h.displayInternal(input)
			h.Quit()

		case "s":
			h.player.LowerVolume()
			h.displayInternal("volume: " + h.player.GetVolume())

		case "w":
			h.player.RaiseVolume()
			h.displayInternal("volume: " + h.player.GetVolume())

		case "m":
			h.player.Mute()
			h.displayInternal("volume: " + h.player.GetVolume())

		case "a", "d":
			h.displayInternal(input)
			offset := 3
			if input == "a" {
				offset *= -1
			}
			err := h.player.SeekRelative(offset)
			if err != nil {
				h.displayInternal(err.Error())
			}

		case "o":
			h.displayInternal(input)
			h.player.Stop()

		case "p":
			h.displayInternal(input)
			h.player.Pause()

		case "r":
			h.playlist.NextMode()
			h.displayInternal("mode: " + h.playlist.GetMode().String())

		case " ":
			h.displayInternal(input)
			h.player.Play()

		case "f":
			h.displayInternal(input)
			h.playlist.Next()

		case "b":
			h.displayInternal(input)
			h.playlist.Prev()

		case ":print progress":
			h.displayInternal(fmt.Sprint(h.player.GetPosition()))

		case ":enqueue data":
			h.displayInternal(fmt.Sprint())
			if err := h.playlist.Enqueue([]item{dummyData}); err != nil {
				h.displayInternal(err.Error())
			}

		case ":add empty":
			h.displayInternal(input)
			if err := h.playlist.Add([]item{}); err != nil {
				h.displayInternal(err.Error())
			}

		case ":add data":
			h.displayInternal(input)
			err := h.playlist.Add([]item{dummyData})
			if err != nil {
				h.displayInternal(err.Error())
			}

		case ":add playlist":
			h.displayInternal(input)
			err := h.playlist.Add([]item{dummyData, dummyData, dummyData, dummyData})
			if err != nil {
				h.displayInternal(err.Error())
			}

		case ":clear data":
			h.displayInternal(input)
			h.playlist.Clear()

		case ":current":
			h.displayInternal(fmt.Sprintf("%+v", h.playlist.GetCurrentItem()))

		default:
			h.displayInternal(input)
			a, dropped, err := parseInput(input)
			if err != nil {
				h.displayInternal(err.Error())
				continue
			}

			if len(dropped) > 0 {
				h.displayInternal(fmt.Sprint(dropped))
			}
			h.do <- a
		}
		h.Update()
	}

	if err := scanner.Err(); err != nil {
		fmt.Println(err)
	}
	h.wg.Done()
}

func newHeadless(player Player, playlist *Playlist) userInterface {
	return &headless{
		formatString: "\x1b[F\x1b[0K%s %s[%s]:\x1b[0m %s%s\n\n",
		prevMessage:  &message{},
		player:       player,
		playlist:     playlist,
	}
}
