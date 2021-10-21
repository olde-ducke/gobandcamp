package main

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/faiface/beep"
)

// TODO: cache cleaning
type cached struct {
	mu    sync.Mutex
	bytes map[int][]byte
}

var cache cached
var player playback
var logFile *os.File

const defaultSampleRate beep.SampleRate = 48000

func checkFatalError(err error) {
	if err != nil {
		app.Quit()
		logFile.WriteString(time.Now().Format(time.ANSIC) + "[err]:" + err.Error())
		// FIXME: can't print while app is finishing
		fmt.Fprintln(os.Stderr, err)
		exitCode = 1
	}
}

func init() {
	var err error
	logFile, err = os.Create("dump.log")
	checkFatalError(err)
}

func main() {
	//
	// FIXME: behaves weird after coming from suspend (high CPU load)
	// FIXME: device does not reinitialize after suspend
	// FIXME: takes device to itself, doesn't allow any other program to use it, and can't use it, if device is already being used
	//player.initPlayer()
	// just switch to SDL, it doesn't have any of these problems
	// FIXME: can't tell orientation on the start for whatever reason
	err := app.Run()
	checkFatalError(err)
	logFile.Close()
	os.Exit(exitCode)
}
