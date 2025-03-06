package main

import (
	"io"
	"log"
	"os"
	"runtime"
	"runtime/pprof"
	"sync"
	"time"
)

var cache *FIFO
var player *beepPlayer
var wg sync.WaitGroup

func init() {
	cache = newCache(4)
}

func run(quit chan int) {
	if err := app.Run(); err != nil {
		log.Printf("[err]: %v", err)
		quit <- 1
	}
	quit <- 0
	wg.Done()
}

func main() {
	code, opt := readOptions()
	if code >= 0 {
		os.Exit(code)
	}

	ticker := time.NewTicker(time.Second)
	quit := make(chan int)
	update := ticker.C
	next := make(chan struct{})
	text := make(chan interface{})

	if opt.cpuProfile != "" {
		file, err := os.Create(opt.cpuProfile)
		if err != nil {
			log.Printf("[err]: %v", err)
			os.Exit(1)
		}
		defer file.Close()
		if err := pprof.StartCPUProfile(file); err != nil {
			log.Printf("[err]: %v", err)
			os.Exit(1)
		}
	}

	player = newBeepPlayer(text, next)

	// TODO: test if needed anymore
	// window.recalculateBounds()
	// TODO: remove wg.Add() from downloaders
	// for now, let them finish gracefully
	wg.Add(1)
	go run(quit)

	// NOTE: this will prevent logging to os.Stderr, while app is running
	// FIXME: this should be reworked completely
	if opt.logFile != nil {
		log.Default().SetOutput(opt.logFile)
	} else {
		log.Default().SetOutput(io.Discard)
	}

loop:
	for {
		select {

		case code = <-quit:
			log.Println("[ext]: main loop exit")
			break loop

		case <-update:
			window.sendEvent(&eventUpdate{})

		case text := <-text:
			switch text := text.(type) {
			case string:
				log.Printf("[dbg]: %s", text)
			case error:
				log.Printf("[err]: %v", text)
				window.sendEvent(newErrorMessage(text))
			}

		case <-next:
			window.sendEvent(&eventNextTrack{})

		default:
			time.Sleep(50 * time.Millisecond)
		}
	}

	ticker.Stop()
	wg.Wait()

	// FIXME: this should be reworked
	if opt.logFile != nil {
		log.Default().SetOutput(io.MultiWriter(os.Stderr, opt.logFile))
	} else {
		log.Default().SetOutput(os.Stderr)
	}

	if opt.cpuProfile != "" {
		pprof.StopCPUProfile()
	}

	if opt.memProfile != "" {
		file, err := os.Create(opt.memProfile)
		if err != nil {
			log.Printf("[err]: %v", err)
			code = 1
		} else {
			defer file.Close()
			runtime.GC()
			err = pprof.WriteHeapProfile(file)
			if err != nil {
				log.Printf("[err]: %v", err)
				code = 1
			}
		}
	}

	if opt.logFile != nil {
		log.Println("[ext]: closing debug file")
		log.Default().SetOutput(os.Stderr)
		if err := opt.logFile.Close(); err != nil {
			code = 1
		}
	}

	os.Exit(code)
}
