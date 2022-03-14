package main

import (
	"flag"
	"log"
	"os"
	"runtime"
	"runtime/pprof"
)

func checkFatalError(err error) {
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}
}

func main() {
	var cpuprofile, memprofile, link string
	var debug, tryhttp bool

	flag.StringVar(&link, "u", "", "play media item")
	flag.StringVar(&link, "url", "", "play media item")
	flag.StringVar(&cpuprofile, "cpuprofile", "", "write cpu profile to `file`")
	flag.StringVar(&memprofile, "memprofile", "", "write memory profile to `file`")
	flag.BoolVar(&debug, "debug", false, "write debug output to 'dump.log'")
	flag.BoolVar(&tryhttp, "tryhttp", true, "try http requests if https fails")
	flag.Parse()

	if cpuprofile != "" {
		file, err := os.Create(cpuprofile)
		checkFatalError(err)
		defer file.Close()
		err = pprof.StartCPUProfile(file)
		checkFatalError(err)
	}

	// TODO: pass tags/urls to run
	run(debug)

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
