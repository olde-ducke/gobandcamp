package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"
)

const playListSize = 1024

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

	context.Canceled = errors.New("download cancelled")

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
	tempCache := newSimpleCache(3)
	extractor := newExtractor(&wg, tempCache, debugln, errorln, text, do)
	musicCache := NewCache(4)
	musicDownloader := newDownloader(&wg, musicCache, debugln, errorln, text, do)
	player := NewBeepPlayer(debugln)
	p := NewPlaylist(player, playListSize, debugln)
	fileManager := newFileManager(musicCache, do)
	Open = fileManager.open
	ui := newHeadless(player, p)
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

			case actionStart:
				data, ok := tempCache.Get(a.path)
				if !ok {
					errorln("incorrect key")
					continue
				}

				items, err := convert(data)
				if err != nil {
					errorln(err.Error())
					continue
				}

				err = p.Add(items)
				if err != nil {
					errorln(err.Error())
					continue
				}

				musicDownloader.run(data[0].tracks[0].mp3128, p.GetCurrentTrack())

			case actionPlay:
				key := getTruncatedURL(a.path)
				// FIXME: might crash
				current := getTruncatedURL(p.GetCurrentItem().Path)

				if key != current {
					debugln("wrong track, discarding")
					continue
				}

				data, ok := musicCache.Get(key)
				if !ok {
					errorln("failed to load data")
					continue
				}
				err := player.Load(data)
				if err != nil {
					errorln(err.Error())
					continue
				}

			case actionDownload:
				musicDownloader.run(a.path, p.GetCurrentTrack())

			case actionQuit:
				ui.Quit()

			default:
				debugln("NOT ALLOWED: unknown action")
			}

			// FIXME: don't remember why this was added
			// default:
			//	time.Sleep(50 * time.Millisecond)
		}
	}
}
