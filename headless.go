package main

import (
	"bufio"
	"fmt"
	"os"
	"sync"
	"time"
)

type headless struct {
	wg           sync.WaitGroup
	formatString string
	prevMessage  message
}

func (h *headless) run(quit chan<- struct{}) {
	h.wg.Add(1)
	go h.start()
	h.wg.Wait()
	h.displayMessage(newMessage(infoMessage, "goodbye"))
	defer close(quit)
}

func (h *headless) update() {
	title, artist := "---", "---"
	t := 0
	total := len(playlist)
	pos := int(player.getCurrentTrackPosition().Seconds())
	if total > 0 {
		t = player.getCurrentTrack()
		title = playlist[t].title
		artist = playlist[t].item.artist
		t++
	}
	// fmt.Print("\x1b[s")
	// fmt.Print("")
	// fmt.Print("\x1b[0K")
	fmt.Printf("\x1b[s\x1b[F\x1b[0K%10s %02d:%02d:%02d \x1b[35m[plr]:\x1b[0m %2s \x1b[35m%s\x1b[0m by \x1b[35m%s\x1b[0m\x1b[u",
		fmt.Sprintf("%d/%d", t, len(playlist)), pos/3600%99, pos/60%60, pos%60,
		player.getStatus(), title, artist)
	// fmt.Print("\x1b[u")
}

func (h *headless) displayMessage(msg *message) {
	var decoration string
	switch msg.msgType {
	case debugMessage:
		decoration = "\x1b[33m"
	case errorMessage:
		decoration = "\x1b[31m"
	case textMessage:
		decoration = "\x1b[36m"
	default:
		decoration = "\x1b[32m"
	}
	fmt.Printf(h.formatString, msg.When().Format("2006/01/02 15:04:05"),
		decoration, msg.msgType, msg.Prefix, msg.text)
	h.update()
}

func (h *headless) start() {
	fmt.Println()
	var input string
	scanner := bufio.NewScanner(os.Stdin)
loop:
	for scanner.Scan() {
		input = scanner.Text()
		fmt.Print("\x1b[F\x1b[0K")
		switch input {
		case "q":
			break loop

		case "s":
			player.lowerVolume()
			h.displayMessage(newMessage(infoMessage, "volume: "+player.getVolume()))
			continue loop

		case "w":
			player.raiseVolume()
			h.displayMessage(newMessage(infoMessage, "volume: "+player.getVolume()))
			continue loop

		case "m":
			player.mute()
			h.displayMessage(newMessage(infoMessage, "volume: "+player.getVolume()))
			continue loop

		case "a", "d":
			player.seek(input == "d")

		case "p":
			player.stop()

		case "r":
			player.nextMode()
			h.displayMessage(newMessage(infoMessage, "mode: "+player.getPlaybackMode()))
			continue loop

		case " ":
			player.playPause()

		// TODO: remove
		case "f":
			if player.skip(true) {
				t := player.currentTrack
				downloader.run(playlist[t].mp3128, t)
			} else {
				dbg("no data")
			}

		case "b":
			if player.getCurrentTrackPosition() > time.Second*5 {
				player.resetPosition()
			} else {
				if player.skip(false) {
					t := player.currentTrack
					downloader.run(playlist[t].mp3128, t)
				} else {
					dbg("no data")
				}
			}

		case "print":
			fmt.Println(playlist[player.getCurrentTrack()].url)

		default:
			handleInput(input)
		}
		h.update()
	}

	if err := scanner.Err(); err != nil {
		fmt.Println(err)
	}
	h.wg.Done()
}

func newHeadless() ui {
	return &headless{formatString: "\x1b[F\x1b[0K%s %s[%s]:\x1b[0m %s%s\n\n"}
}
