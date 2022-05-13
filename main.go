package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"runtime"
	"runtime/pprof"
)

const (
	version = "0.0.1"
	verText = `
gobandcamp version:       %s
available sound backends: %s
`
)

type config struct {
	cpuprofile string
	memprofile string
	link       string
	snd        string
	debug      bool
	tryhttp    bool
	v          bool
}

func checkFatalError(err error) {
	if err != nil {
		log.Fatalf("\x1b[31m[err]:\x1b[0m %v", err)
	}
}

func main() {
	var cfg config

	flag.StringVar(&cfg.link, "u", "", "play media item")
	flag.StringVar(&cfg.link, "url", "", "play media item")
	flag.StringVar(&cfg.cpuprofile, "cpuprofile", "", "write cpu profile to `file`")
	flag.StringVar(&cfg.memprofile, "memprofile", "", "write memory profile to `file`")
	flag.StringVar(&cfg.snd, "snd", "beep", "backend for audio playback")
	flag.BoolVar(&cfg.v, "v", false, "print version and exit")
	flag.BoolVar(&cfg.v, "version", false, "print version and exit")
	flag.BoolVar(&cfg.debug, "debug", false, "write debug output to 'dump.log'")
	flag.BoolVar(&cfg.tryhttp, "tryhttp", true, "try http requests if https fails")
	flag.Parse()

	if cfg.v {
		fmt.Printf(verText, version, AvailableBackends())
		os.Exit(0)
	}

	if cfg.cpuprofile != "" {
		file, err := os.Create(cfg.cpuprofile)
		checkFatalError(err)
		defer checkFatalError(file.Close())
		err = pprof.StartCPUProfile(file)
		checkFatalError(err)
	}

	run(cfg)

	if cfg.cpuprofile != "" {
		pprof.StopCPUProfile()
	}

	if cfg.memprofile != "" {
		file, err := os.Create(cfg.memprofile)
		checkFatalError(err)
		defer checkFatalError(file.Close())
		runtime.GC()
		err = pprof.WriteHeapProfile(file)
		checkFatalError(err)
	}

	os.Exit(0)
}
