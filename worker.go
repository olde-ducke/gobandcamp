package main

import (
	"context"
	"sync"
)

type workerType int

const (
	downloader workerType = iota
	extractor
	searcher
)

type worker interface {
	stop()
	cancelPrevJob(func())
	run(string, func(string, ...any))
}

type blankWorker struct {
	sync.Mutex
	cache      *FIFO
	cancelCurr func()
	errorf     func(string, ...any)
	wg         *sync.WaitGroup
	do         chan<- *action
}

func (w *blankWorker) stop() {
	w.cancelCurr()
}

func (w *blankWorker) cancelPrevJob(cancel func()) {
	w.Lock()
	w.cancelCurr()
	w.cancelCurr = cancel
	w.Unlock()
}

type extractorWorker struct {
	*blankWorker
}

func (w *extractorWorker) run(link string, infof func(string, ...any)) {
	ctx, cancel := context.WithCancel(context.Background())
	w.cancelPrevJob(cancel)

	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		items, err := processmediapage(ctx, link, infof)
		if err != nil {
			w.errorf(err.Error())
			return
		}

		w.cache.Set(link, items)
		w.do <- &action{actionStart, link}
	}()
}

type downloadWorker struct {
	*blankWorker
}

func (w *downloadWorker) run(link string, infof func(string, ...any)) {
	ctx, cancel := context.WithCancel(context.Background())
	w.cancelPrevJob(cancel)

	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		result, err := downloadmedia(ctx, link, infof)
		if err != nil {
			if err == context.Canceled {
				infof(err.Error())
				return
			}
			w.errorf(err.Error())
			return
		}

		key := getTruncatedURL(link)
		if key == "" {
			w.errorf("incorrect cache key")
			return
		}

		w.cache.Set(key, result)
		infof("downloaded")
		w.do <- &action{actionPlay, key}
	}()
}

func newWorker(t workerType,
	cache *FIFO,
	errorf func(string, ...any),
	wg *sync.WaitGroup,
	do chan<- *action) worker {

	w := &blankWorker{
		cache:      cache,
		cancelCurr: func() {},
		errorf:     errorf,
		wg:         wg,
		do:         do,
	}

	switch t {
	case downloader:
		return &downloadWorker{w}
	case extractor:
		return &extractorWorker{w}
	default:
		errorf("unexpected worker type, will fail")
		return nil
	}
}
