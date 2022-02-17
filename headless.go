package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"sync"
	"time"
)

type headless struct {
	wg           sync.WaitGroup
	formatString string
}

func (h *headless) run(quit chan<- struct{}) {
	h.wg.Add(1)
	go h.start()
	h.wg.Wait()
	log.Printf(h.formatString, "\x1b[32m", "ext",
		"", "goodbye")
	defer close(quit)
}

func (h *headless) update() {
	if len(playlist) > 0 {
		t := player.getCurrentTrack()
		fmt.Printf("\r \x1b[37m%s\x1b[0m %d/%d %s by %s %s\r", player.getStatus(), t+1,
			len(playlist),
			playlist[t].title, playlist[t].item.artist,
			player.getCurrentTrackPosition().Round(time.Second))
	} else {
		fmt.Printf("\r \x1b[37m%s\x1b[0m %d/%d %s by %s %s\r", player.getStatus(),
			0, 0, "---", "---", "")
	}
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
	}
	log.Printf(h.formatString, decoration, msg.msgType,
		msg.Prefix, msg.text)
}

func (h *headless) start() {
	var input string
	scanner := bufio.NewScanner(os.Stdin)
loop:
	for scanner.Scan() {
		input = scanner.Text()
		switch input {
		case "q":
			break loop
		case "f", "b":
			player.skip(true)
			if len(playlist) > 0 {
				t := player.currentTrack
				downloader.stop()
				downloader.run(playlist[t].mp3128, t)
			} else {
				dbg("no data")
			}
		case "p":
			if len(playlist) > 0 {
				fmt.Println(playlist)
			} else {
				dbg("no data")
			}
		default:
			handleInput(input)
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Println(err)
	}
	h.wg.Done()
}

func newHeadless() ui {
	return &headless{formatString: "%s[%s]:\x1b[0m %s%s\n"}
}
