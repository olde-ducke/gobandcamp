package main

import (
	"context"
	"errors"
	"sync"

	"github.com/olde-ducke/gobandcamp/player"
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
	storage    Storage
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

		w.storage.Set(link, items)
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
		data, contentType, err := downloadmedia(ctx, link, infof)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				infof(err.Error())
				return
			}
			w.errorf(err.Error())
			return
		}

		key := getTruncatedURL(link)
		w.storage.Set(key, &player.Media{Data: data, ContentType: contentType})
		infof("downloaded")
		w.do <- &action{actionPlay, key}
	}()
}

func newWorker(t workerType,
	storage Storage,
	errorf func(string, ...any),
	wg *sync.WaitGroup,
	do chan<- *action) worker {

	w := &blankWorker{
		storage:    storage,
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
		panic("unexpected worker type, will fail")
	}
}
