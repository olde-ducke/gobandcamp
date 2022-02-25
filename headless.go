package main

import (
	"bufio"
	"fmt"
	"os"
	"sync"
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
	pos := int(player.getTime().Seconds())
	if total > 0 {
		// t = player.getCurrentTrack()
		// title = playlist[t].title
		// artist = playlist[t].item.artist
		// t++
	}
	fmt.Printf("\x1b[s\x1b[F\x1b[0K%10s %02d:%02d:%02d \x1b[35m[plr]:\x1b[0m %2s \x1b[35m%s\x1b[0m by \x1b[35m%s\x1b[0m\x1b[u",
		fmt.Sprintf("%d/%d", t, len(playlist)), pos/3600%99, pos/60%60, pos%60,
		player.getStatus(), title, artist)
}

func (h *headless) displayInternal(text string) {
	h.displayMessage(newMessage(infoMessage, text))
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
	h.update()
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
			h.displayInternal("volume: " + player.getVolume())
			continue loop

		case "w":
			player.raiseVolume()
			h.displayInternal("volume: " + player.getVolume())
			continue loop

		case "m":
			player.mute()
			h.displayInternal("volume: " + player.getVolume())
			continue loop

		case "a", "d":
			offset := 3
			if input == "a" {
				offset *= -1
			}
			err := player.seekRelative(offset)
			if err != nil {
				h.displayInternal(err.Error())
			}

		case "o":
			player.stop()

		case "p":
			player.pause()

		case "r":
			// TODO: add playback mode switch
			h.displayInternal("NOTE: NOT IMPLEMENTED")
			continue loop

		case " ":
			player.playPause()

		case "f":
			// TODO: add next()
			h.displayInternal("NOTE: NOT IMPLEMENTED")
			continue loop

		case "b":
			// TODO: add prev/reset()
			h.displayInternal("NOTE: NOT IMPLEMENTED")
			continue loop

			// TODO: add playlist/lyrics/other metadata printing

		case "print progress":
			h.displayInternal(fmt.Sprint(player.getPosition()))

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
