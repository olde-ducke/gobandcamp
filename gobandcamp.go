package main

import (
	"fmt"
	"os"
	"sync"
	"time"
)

func run(debug bool) {
	ticker := time.NewTicker(time.Second)
	update := ticker.C
	quit := make(chan struct{})
	input := make(chan string)
	finish := make(chan struct{})
	text := make(chan *message)

	var logFile *os.File
	var wg sync.WaitGroup
	var err error

	// open file to write logs if needed and create
	// debug logging function, if debug is set to false
	// dbg function is empty and won't fill queue with
	// messages
	var dbg func(string)
	if debug {
		logFile, err = os.Create("dump.log")
		checkFatalError(err)
		dbg = newReporter(debugMessage, "", &wg, text)
	} else {
		dbg = func(msg string) {}
	}

	errr := newReporter(errorMessage, "", &wg, text)

	var quitting bool
	player := NewBeepPlayer(dbg)
	p := NewPlaylist(player, dbg)
	ui := newHeadless(player, p)
	tempCache := newSimpleCache(3)
	extractor := newExtractor(&wg, tempCache, dbg, errr, text)
	musicCache := NewCache(4)
	musicDownloader := newDownloader(&wg, musicCache, dbg, errr, text)
	// FIXME: no wg on run, ui should dictate when to finish
	// so it's probably fine?
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
				defer close(finish)
				wg.Wait()
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
				errr(err.Error())
				// TODO: delete later
				if len(dropped) > 0 {
					dbg(fmt.Sprint("dropped arguments: ", dropped))
				}
				continue
			}

			if len(dropped) > 0 {
				dbg(fmt.Sprint("dropped arguments: ", dropped))
			}
			dbg(fmt.Sprintf("%+v", parsed))
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
