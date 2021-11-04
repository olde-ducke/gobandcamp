package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"runtime/pprof"
	"time"
)

// TODO: move to app
var exitCode = 0

var cache *FIFO
var player playback
var logFile *os.File

func checkFatalError(err error) {
	if err != nil {
		app.Quit()
		if *debug {
			logFile.WriteString(time.Now().Format(time.ANSIC) + "[err]:" + err.Error())
		}
		// FIXME: can't print while app is finishing
		// sometimes does print though
		fmt.Fprintln(os.Stderr, err)
		exitCode = 1
	}
}

func init() {
	cache = newCache(4)
}

// TODO: exit loop
func updater(quit chan bool, update <-chan time.Time) {
	for {
		select {
		case <-quit:
			return
		case <-update:
			window.sendEvent(nil)
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

	// FIXME: behaves weird after coming from suspend (high CPU load)
	// FIXME: device does not reinitialize after suspend
	// FIXME: takes device to itself, doesn't allow any other program to use it, and can't use it, if device is already being used
	// just switch to SDL, it doesn't have any of these problems
	// FIXME: can't tell orientation on the start for whatever reason

	err := app.Run()
	checkFatalError(err)

	if *memprofile != "" {
		file, err := os.Create(*memprofile)
		checkFatalError(err)
		defer file.Close()
		runtime.GC()
		err = pprof.WriteHeapProfile(file)
		checkFatalError(err)
	}

	quit <- true
	close(quit)
	ticker.Stop()
	time.Sleep(time.Second / 3)

	if *debug {
		logFile.Close()
		pprof.StopCPUProfile()
	}

	os.Exit(exitCode)
}
