package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"runtime/pprof"
	"sync"
	"time"
)

// TODO: move to app?
var exitCode = 0

var cache *FIFO
var player = playback{timeStep: 2}
var logFile *os.File
var wg sync.WaitGroup

func checkFatalError(err error) {
	if err != nil {
		app.Quit()
		if *debug {
			logFile.WriteString(time.Now().Format(time.ANSIC) + "[err]:" + err.Error())
		}
		fmt.Fprintln(os.Stderr, err)
		exitCode = 1
	}
}

func init() {
	cache = newCache(4)
}

func updater(quit chan bool, update <-chan time.Time) {
	wg.Add(1)
	for {
		select {

		case <-quit:
			if *debug {
				logFile.WriteString(time.Now().Format(time.ANSIC) + "[ext]:updater loop exit\n")
			}
			wg.Done()
			return

		case <-update:
			window.sendEvent(&eventUpdate{})
			if player.status == seekBWD || player.status == seekFWD {
				player.status = player.bufferedStatus
			}
		}
	}
}

// TODO: combine with input args
var cpuprofile = flag.String("cpuprofile", "", "write cpu profile to `file`")
var memprofile = flag.String("memprofile", "", "write memory profile to `file`")
var debug = flag.Bool("debug", false, "write debug output to 'dump.log'")

func main() {
	ticker := time.NewTicker(time.Second)
	quit := make(chan bool)
	update := ticker.C
	go updater(quit, update)

	flag.Parse()

	if *debug {
		var err error
		logFile, err = os.Create("dump.log")
		checkFatalError(err)
	}

	if *cpuprofile != "" {
		file, err := os.Create(*cpuprofile)
		checkFatalError(err)
		defer file.Close()
		err = pprof.StartCPUProfile(file)
		checkFatalError(err)
	}

	window.recalculateBounds()
	// FIXME: behaves weird after coming from suspend (high CPU load)
	// FIXME: device does not reinitialize after suspend
	// FIXME: takes device to itself, doesn't allow any other program to use it, and can't use it, if device is already being used
	// just switch to SDL, it doesn't have any of these problems
	err := app.Run()
	checkFatalError(err)

	// exit updater loop, close channel, stop timer
	// wait for them to finish
	quit <- true
	close(quit)
	ticker.Stop()
	wg.Wait()

	if *debug {
		logFile.WriteString(time.Now().Format(time.ANSIC) + "[ext]:closing debug file\n")
		err = logFile.Close()
		if err != nil {
			exitCode = 1
		}
	}

	if *cpuprofile != "" {
		pprof.StopCPUProfile()
	}

	if *memprofile != "" {
		file, err := os.Create(*memprofile)
		checkFatalError(err)
		defer file.Close()
		runtime.GC()
		err = pprof.WriteHeapProfile(file)
		checkFatalError(err)
	}

	os.Exit(exitCode)
}
