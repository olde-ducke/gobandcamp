package main

import (
	"fmt"
	"os"
	"sync"
	"time"
)

var text = make(chan *message)

func run(debug bool) {
	ticker := time.NewTicker(time.Second)
	update := ticker.C
	quit := make(chan struct{})
	input := make(chan string)
	finish := make(chan struct{})

	var logFile *os.File
	var wg sync.WaitGroup
	var err error

	// open file to write logs if needed and create
	// debug logging function, if debug is set to false
	// dbg function is empty and won't fill queue with
	// messages
	dbg := func(msg string) {}
	if debug {
		logFile, err = os.Create("dump.log")
		checkFatalError(err)

		dbg = func(str string) {
			msg := newMessage(debugMessage, str)
			// NOTE: new goroutines will be started within goroutines
			// might break something
			wg.Add(1)
			go func() {
				text <- msg
				defer wg.Done()
			}()
		}
	}

	var quitting bool
	player := NewBeepPlayer(dbg)
	p := NewPlaylist(player, dbg)
	ui := newHeadless(player, p)
	extractor := newExtractor(&wg, p, dbg)
	musicCache := NewCache(4)
	musicDownloader := newDownloader(&wg, musicCache, dbg)
	go ui.Run(quit, input)

loop:
	for {
		select {

		case <-quit:
			if quitting {
				continue
			}
			dbg("got signal to finish")
			quitting = true
			ticker.Stop()
			extractor.stop()
			musicDownloader.stop()
			player.ClearStream()

			// wait for other goroutines and send final signal
			go func() {
				wg.Wait()
				defer close(finish)
			}()

		case <-finish:
			break loop

		case <-update:
			ui.Update()

		case msg := <-text:
			ui.DisplayMessage(msg)
			if logFile != nil {
				logFile.WriteString(msg.String())
			}

		case input := <-input:
			parsed, dropped, err := parseInput(input)
			if err != nil {
				go func() {
					text <- newMessage(errorMessage, err.Error())
				}()

				if len(dropped) > 0 {
					dbg(fmt.Sprint("dropped arguments: ", dropped))
				}
				continue
			}

			if len(dropped) > 0 {
				dbg(fmt.Sprint("dropped arguments: ", dropped))
			}
			dbg(parsed.String())
			switch parsed.action {

			case actionSearch:
				dbg("NOT IMPLEMENTED: actionSearch")

			case actionTagSearch:
				dbg("NOT IMPLEMENTED: actionTagSearch")

			case actionOpen:
				dbg("NOT IMPLEMENTED: actionOpen")

			case actionOpenURL:
				extractor.run(parsed.path)

			case actionAdd:
				dbg("NOT IMPLEMENTED: actionAdd")

			case actionQuit:
				ui.Quit()
			}

			// FIXME: don't remember why this was added
			// default:
			//	time.Sleep(50 * time.Millisecond)
		}
	}
}
