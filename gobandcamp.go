package main

import (
	"context"
	"errors"
	"os"
	"sync"
	"time"

	"github.com/olde-ducke/gobandcamp/player"
)

const playListSize = 1024

func run(cfg config) {
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

	debugf := func(string, ...any) {}
	errorf := newReporter(errorMessage, "", &wg, text)
	// open file to write logs if needed and create
	// debug logging function, if debug is set to false
	// dbg function is empty and won't fill queue with
	// messages
	if cfg.debug {
		debugf = newReporter(debugMessage, "", &wg, text)
		logFile, err = os.Create("dump.log")
		checkFatalError(err)
		defer func() {
			msg := newMessage(debugMessage, "gobandcamp: ", "closing log file")
			_, err := logFile.WriteString(msg.String())
			checkFatalError(err)
			checkFatalError(logFile.Close())
		}()
		player.Debugf = debugf
		Debugf = debugf
		debugf("%+v", cfg)
	}

	var quitting bool
	tempCache := newSimpleCache(3)
	extractor := newExtractor(&wg, tempCache, debugf, errorf, text, do)
	musicCache := NewCache(4)
	musicDownloader := newDownloader(&wg, musicCache, debugf, errorf, text, do)
	p, err := player.NewPlayer(cfg.snd)
	checkFatalError(err)

	fm := newFileManager(musicCache, do)
	pl := player.NewPlaylist(p, playListSize, fm.open)
	ui := newHeadless(p, pl)
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
			debugf("got signal to finish")
			quitting = true
			ticker.Stop()
			extractor.stop()
			musicDownloader.stop()
			p.ClearStream()

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
			debugf("%+v", a)
			switch a.actionType {

			case actionSearch:
				debugf("NOT IMPLEMENTED: actionSearch")

			case actionTagSearch:
				debugf("NOT IMPLEMENTED: actionTagSearch")

			case actionOpen:
				debugf("NOT IMPLEMENTED: actionOpen")

			case actionOpenURL:
				extractor.run(a.path)

			case actionAdd:
				debugf("NOT IMPLEMENTED: actionAdd")

			case actionStart:
				data, ok := tempCache.Get(a.path)
				if !ok {
					errorf("incorrect key")
					continue
				}

				items, err := convert(data...)
				if err != nil {
					errorf(err.Error())
					continue
				}

				err = pl.New(items)
				if err != nil {
					errorf(err.Error())
					continue
				}

				musicDownloader.run(data[0].tracks[0].mp3128, pl.GetCurrentTrack())

			case actionPlay:
				key := getTruncatedURL(a.path)
				// FIXME: might crash
				current := getTruncatedURL(pl.GetCurrentItem().Path)

				if key != current {
					debugf("wrong track, discarding")
					continue
				}

				data, ok := musicCache.Get(key)
				if !ok {
					errorf("failed to load data")
					continue
				}
				err := p.Load(data)
				if err != nil {
					errorf(err.Error())
					continue
				}

			case actionDownload:
				musicDownloader.run(a.path, pl.GetCurrentTrack())

			case actionQuit:
				ui.Quit()

			default:
				debugf("NOT ALLOWED: unknown action")
			}

			// FIXME: don't remember why this was added
			// default:
			//	time.Sleep(50 * time.Millisecond)
		}
	}
}
