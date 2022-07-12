package main

import (
	"context"
	"sync"
)

type worker struct {
	sync.Mutex
	cancelCurr func()
	errorf     func(string, ...any)
	wg         *sync.WaitGroup
	do         chan<- *action
}

func (w *worker) stop() {
	w.cancelCurr()
}

func (w *worker) cancelPrevJob(cancel func()) {
	w.Lock()
	w.cancelCurr()
	w.cancelCurr = cancel
	w.Unlock()
}

func newWorker(wg *sync.WaitGroup, errorf func(string, ...any), do chan<- *action) *worker {
	return &worker{
		cancelCurr: func() {},
		errorf:     errorf,
		wg:         wg,
		do:         do,
	}
}

type extractorWorker struct {
	*worker
	cache *simpleCache
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

func newExtractor(wg *sync.WaitGroup, cache *simpleCache, errorf func(string, ...any), do chan<- *action) *extractorWorker {
	return &extractorWorker{
		worker: newWorker(wg, errorf, do),
		cache:  cache,
	}
}

type downloadWorker struct {
	*worker
	cache *FIFO
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

func newDownloader(wg *sync.WaitGroup, cache *FIFO, errorf func(string, ...any), do chan<- *action) *downloadWorker {
	return &downloadWorker{
		worker: newWorker(wg, errorf, do),
		cache:  cache,
	}
}
