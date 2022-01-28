package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"runtime"
	"runtime/pprof"
	"sync"
	"time"
)

var cache *FIFO
var player *beepPlayer

func checkFatalError(err error) {
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}
}

func init() {
	cache = newCache(4)
}

type ui interface {
	run(quit chan<- struct{})
	update()
	displayMessage(*message)
}

/*func run(quit chan struct{}) {
	err := app.Run()
	checkFatalError(err)
	quit <- struct{}{}
	wg.Done()
}*/

// TODO: move to different file
const formatString = "%s [%s]: %s%s\n"

type messageType int

const (
	debugMessage messageType = iota
	errorMessage
	textMessage
)

var types = [3]string{"dbg", "err", "msg"}

func (t messageType) String() string {
	return types[t]
}

type message struct {
	Prefix string

	msgType   messageType
	text      string
	timestamp time.Time
}

func (msg *message) When() time.Time {
	return msg.timestamp
}

func (msg *message) Type() messageType {
	return msg.msgType
}

func (msg *message) String() string {
	return fmt.Sprintf(formatString, msg.timestamp.Format(time.ANSIC),
		msg.msgType, msg.Prefix, msg.text)
}

func (msg *message) Text() string {
	return msg.Prefix + msg.text
}

func newMessage(t messageType, str string) *message {
	return &message{
		msgType:   t,
		text:      str,
		timestamp: time.Now(),
	}
}

// ***************************

func main() {
	var cpuprofile, memprofile, link, tag, sort, format string
	var debug, tryhttp bool

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
	flag.BoolVar(&tryhttp, "tryhttp", true, "try http requests if https fails")
	flag.Parse()

	ticker := time.NewTicker(time.Second)
	update := ticker.C
	next := make(chan struct{})
	quit := make(chan struct{})
	finish := make(chan struct{})
	text := make(chan *message)

	var quitting bool
	var wg sync.WaitGroup
	var logFile *os.File
	var err error
	var dbg func(string)

	// open file to write logs if needed and create
	// debug logging function, if debug is set to false
	// dbg function is empty and won't fill queue with
	// messages
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
	} else {
		dbg = func(msg string) {}
	}

	if cpuprofile != "" {
		file, err := os.Create(cpuprofile)
		checkFatalError(err)
		defer file.Close()
		err = pprof.StartCPUProfile(file)
		checkFatalError(err)
	}

	// if mediaURL, ok := isValidURL(link); ok {
	// wg.Add(1)
	// go processMediaPage(mediaURL, text)
	// }

	if tag != "" {
		/*
			sort = filterSort(sort)
			format = filterFormat(format)

			wg.Add(1)
			go processTagPage(&arguments{
				tags:   strings.Fields(tag),
				sort:   sort,
				format: format,
			}, text)
		*/
	}

	player = newBeepPlayer(dbg, next)
	userInterface := newHeadless()

	//########################
	// throw away

	prefix := "prefix "
	ctx, cancel := context.WithCancel(context.Background())

	wg.Add(1)
	go func() {
		defer wg.Done()
		link = "https://modestmouse.bandcamp.com"
		result, err := processmediapage(ctx, link, dbg,
			func(str string) {
				msg := newMessage(textMessage, str)
				msg.Prefix = prefix
				// do not wait for other end, send and forget
				wg.Add(1)
				go func() {
					text <- msg
					defer wg.Done()
				}()
			})
		if err != nil {
			msg := newMessage(errorMessage, err.Error())
			text <- msg
		} else {
			msg := newMessage(textMessage, result)
			msg.Prefix = prefix
			text <- msg
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
			if quitting {
				continue
			}
			dbg("got signal to finish")
			quitting = true
			ticker.Stop()
			cancel()

			// wait for other goroutines and send final signal
			go func() {
				wg.Wait()
				defer close(finish)
			}()

		case <-finish:
			break loop

		case <-update:
			// TODO: replace with app.Update ???
			// TODO: consider switching event sending
			// to app and defining app as interface
			// window.sendEvent(&eventUpdate{})
			// logger.writeToLogFile("[upd]: update")
			userInterface.update()

		case msg := <-text:
			userInterface.displayMessage(msg)
			if logFile != nil {
				logFile.WriteString(msg.String())
			}

		case <-next:
			log.Println("next")

		default:
			time.Sleep(50 * time.Millisecond)
		}
	}

	if debug {
		logFile.Close()
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
