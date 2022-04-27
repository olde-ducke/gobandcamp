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
	do := make(chan *action)
	finish := make(chan struct{})
	text := make(chan *message)

	var logFile *os.File
	var wg sync.WaitGroup
	var err error

	debugln := func(string) {}
	errorln := newReporter(errorMessage, "", &wg, text)
	// open file to write logs if needed and create
	// debug logging function, if debug is set to false
	// dbg function is empty and won't fill queue with
	// messages
	if debug {
		debugln = newReporter(debugMessage, "", &wg, text)
		logFile, err = os.Create("dump.log")
		checkFatalError(err)
		defer func() {
			msg := newMessage(debugMessage, "gobandcamp: ", "closing log file")
			logFile.WriteString(msg.String())
			checkFatalError(logFile.Close())
		}()
	}

	var quitting bool
	player := NewBeepPlayer(debugln)
	p := NewPlaylist(player, debugln)
	ui := newHeadless(player, p)
	tempCache := newSimpleCache(3)
	extractor := newExtractor(&wg, tempCache, debugln, errorln, text)
	musicCache := NewCache(4)
	musicDownloader := newDownloader(&wg, musicCache, debugln, errorln, text)
	// FIXME: no wg on run, ui should dictate when to finish
	// so it's probably fine?
	go ui.Run(quit, do)

loop:
	for {
		select {

		case <-quit:
			if quitting {
				continue
			}
			debugln("got signal to finish")
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

		case a := <-do:
			debugln(fmt.Sprintf("%+v", a))
			switch a.actionType {

			case actionSearch:
				debugln("NOT IMPLEMENTED: actionSearch")

			case actionTagSearch:
				debugln("NOT IMPLEMENTED: actionTagSearch")

			case actionOpen:
				debugln("NOT IMPLEMENTED: actionOpen")

			case actionOpenURL:
				extractor.run(a.path)

			case actionAdd:
				debugln("NOT IMPLEMENTED: actionAdd")

			case actionQuit:
				ui.Quit()
			}

			// FIXME: don't remember why this was added
			// default:
			//	time.Sleep(50 * time.Millisecond)
		}
	}
}
