package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"runtime"
	"runtime/pprof"
	"strings"
	"sync"
	"time"
)

var cache *FIFO
var player *beepPlayer
var wg sync.WaitGroup

func checkFatalError(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	cache = newCache(4)
}

type ui interface {
	run(quit chan<- struct{})
	update()
	displayMessage(string)
}

/*func run(quit chan struct{}) {
	err := app.Run()
	checkFatalError(err)
	quit <- struct{}{}
	wg.Done()
}*/

func main() {
	var cpuprofile, memprofile, link, tag, sort, format string
	var debug bool
	flag.StringVar(&link, "u", "", "play media item")
	flag.StringVar(&link, "url", "", "play media item")
	flag.StringVar(&tag, "t", "", "tags for tag search")
	flag.StringVar(&tag, "tag", "", "tags for tag search")
	flag.StringVar(&sort, "s", "", "sorting method")
	flag.StringVar(&sort, "sort", "", "sorting method")
	flag.StringVar(&format, "f", "", "physical format")
	flag.StringVar(&format, "format", "", "physical format")
	flag.StringVar(&cpuprofile, "cpuprofile", "", "write cpu profile to `file`")
	flag.StringVar(&memprofile, "memprofile", "", "write memory profile to `file`")
	flag.BoolVar(&debug, "debug", false, "write debug output to 'dump.log'")
	flag.BoolVar(&debug, "tryhttp", true, "try http requests if https fails")
	flag.Parse()

	ticker := time.NewTicker(time.Second)
	update := ticker.C
	next := make(chan struct{})
	quit := make(chan struct{})
	message := make(chan interface{})

	var logger debugLogger

	if debug {
		err := logger.initialize()
		checkFatalError(err)
	}

	if cpuprofile != "" {
		file, err := os.Create(cpuprofile)
		checkFatalError(err)
		defer file.Close()
		err = pprof.StartCPUProfile(file)
		checkFatalError(err)
	}

	if mediaURL, ok := isValidURL(link); ok {
		wg.Add(1)
		go processMediaPage(mediaURL, message)
	}

	if tag != "" {
		sort = filterSort(sort)
		format = filterFormat(format)

		wg.Add(1)
		go processTagPage(&arguments{
			tags:   strings.Fields(tag),
			sort:   sort,
			format: format,
		}, message)

	}

	player = newBeepPlayer(message, next)
	userInterface := newHeadless()

	//########################
	// throw away
	go func() {
		ctx, _ := context.WithCancel(context.Background())
		link = "https://modestmouse.bandcamp.com"
		result, err := processmediapage(ctx, link, func(dbg string) { message <- dbg },
			func(msg string) { message <- "album art 1 " + msg })
		if err != nil {
			message <- err
		} else {
			message <- result
		}
	}()
	//##############################

	// TODO: test if needed anymore
	// window.recalculateBounds()

	// FIXME: behaves weird after coming from suspend (high CPU load)
	// FIXME: device does not reinitialize after suspend
	// FIXME: takes device to itself, doesn't allow any other program to use it, and can't use it, if device is already being used

	// TODO: remove wg.Add() from downloaders
	// for now, let them finish gracefully

	go userInterface.run(quit)

loop:
	for {
		select {

		case <-quit:
			logger.writeToLogFile("[ext]: main loop exit")
			log.Println("\x1b[32m[ext]:\x1b[0m main loop exit")
			break loop

		case <-update:
			// TODO: replace with app.Update ???
			// TODO: consider switching event sending
			// to app and defining app as interface
			// window.sendEvent(&eventUpdate{})
			// log.Println("[upd]: update")
			// logger.writeToLogFile("[upd]: update")
			userInterface.update()

		case text := <-message:
			switch text := text.(type) {

			case string:
				logger.writeToLogFile("[dbg]: " + text)
				if debug {
					userInterface.displayMessage("\x1b[33m[dbg]:\x1b[0m " + text)
				}

			case error:
				logger.writeToLogFile("[err]: " + text.Error())
				// window.sendEvent(newErrorMessage(text))
				// log.Println("[err]: " + text.Error())
				userInterface.displayMessage("\x1b[31m[err]\x1b[0m: " + text.Error())

			case *info:
				logger.writeToLogFile("[msg]: " + text.String())
				// log.Println("[msg]: " + text.String())
				userInterface.displayMessage("\x1b[36m[msg]:\x1b[0m " + text.String())
			}

		case <-next:
			// window.sendEvent(&eventNextTrack{})
			log.Println("next")

		default:
			time.Sleep(50 * time.Millisecond)
		}
	}

	close(message)
	ticker.Stop()
	wg.Wait()

	if debug {
		// logger.writeToLogFile("[ext]: closing debug file")
		logger.finalize()
	}

	if cpuprofile != "" {
		pprof.StopCPUProfile()
	}

	if memprofile != "" {
		file, err := os.Create(memprofile)
		checkFatalError(err)
		runtime.GC()
		err = pprof.WriteHeapProfile(file)
		checkFatalError(err)
		file.Close()
	}

	os.Exit(0)
}
