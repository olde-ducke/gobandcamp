package main

import (
	"context"
	"strconv"
	"sync"
)

type worker struct {
	cancelPrev func()
	cancelCurr func()
	errorf     func(string, ...any)
	wg         *sync.WaitGroup
	out        chan<- *message
	do         chan<- *action
}

func (w *worker) stop() {
	w.cancelPrev()
	w.cancelCurr()
}

func (w *worker) cancelPrevJob(cancel func()) {
	w.cancelPrev = w.cancelCurr
	w.cancelPrev()
	w.cancelCurr = cancel
}

func newWorker(wg *sync.WaitGroup, errorf func(string, ...any), out chan<- *message, do chan<- *action) *worker {
	return &worker{
		cancelPrev: func() {},
		cancelCurr: func() {},
		errorf:     errorf,
		wg:         wg,
		out:        out,
		do:         do,
	}
}

type extractorWorker struct {
	*worker
	cache *simpleCache
}

func (w *extractorWorker) run(link string) {
	ctx, cancel := context.WithCancel(context.Background())
	w.cancelPrevJob(cancel)

	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		items, err := processmediapage(ctx, link,
			newReporter(textMessage, link+" ", w.wg, w.out))
		if err != nil {
			w.errorf(err.Error())
			return
		}

		w.cache.Set(link, items)
		w.do <- &action{actionStart, link}
	}()
}

func newExtractor(wg *sync.WaitGroup, cache *simpleCache, errorf func(string, ...any), out chan<- *message, do chan<- *action) *extractorWorker {
	return &extractorWorker{
		worker: newWorker(wg, errorf, out, do),
		cache:  cache,
	}
}

type downloadWorker struct {
	*worker
	cache *FIFO
}

func (w *downloadWorker) run(link string, n int) {
	ctx, cancel := context.WithCancel(context.Background())
	w.cancelPrevJob(cancel)

	prefix := "track " + strconv.Itoa(n) + " "
	infof := newReporter(textMessage, prefix, w.wg, w.out)

	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		result, err := downloadmedia(ctx, link, infof)
		if err != nil {
			if err == context.Canceled {
				infof(err.Error())
				return
			}
			w.errorf(prefix + err.Error())
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

func newDownloader(wg *sync.WaitGroup, cache *FIFO, errorf func(string, ...any), out chan<- *message, do chan<- *action) *downloadWorker {
	return &downloadWorker{
		worker: newWorker(wg, errorf, out, do),
		cache:  cache,
	}
}
