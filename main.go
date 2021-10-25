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
		logFile.WriteString(time.Now().Format(time.ANSIC) + "[err]:" + err.Error())
		// FIXME: can't print while app is finishing
		// sometimes does print though
		fmt.Fprintln(os.Stderr, err)
		exitCode = 1
	}
}

func init() {
	var err error
	logFile, err = os.Create("dump.log")
	checkFatalError(err)
	cache = newCache(4)
}

var cpuprofile = flag.String("cpuprofile", "", "write cpu profile to `file`")
var memprofile = flag.String("memprofile", "", "write memory profile to `file`")

func main() {

	flag.Parse()
	if *cpuprofile != "" {
		file, err := os.Create(*cpuprofile)
		checkFatalError(err)
		defer file.Close()
		err = pprof.StartCPUProfile(file)
		checkFatalError(err)
	}

	go func() {
		for {
			time.Sleep(time.Second / 2)
			window.sendPlayerEvent(nil)
			if player.status == seekBWD || player.status == seekFWD {
				player.status = player.bufferedStatus
			}
		}
	}()
	// FIXME: behaves weird after coming from suspend (high CPU load)
	// FIXME: device does not reinitialize after suspend
	// FIXME: takes device to itself, doesn't allow any other program to use it, and can't use it, if device is already being used
	// just switch to SDL, it doesn't have any of these problems
	// FIXME: can't tell orientation on the start for whatever reason
	err := app.Run()
	checkFatalError(err)

	logFile.Close()

	if *memprofile != "" {
		file, err := os.Create(*memprofile)
		checkFatalError(err)
		defer file.Close()
		runtime.GC()
		err = pprof.WriteHeapProfile(file)
		checkFatalError(err)
	}
	pprof.StopCPUProfile()
	os.Exit(exitCode)
}
