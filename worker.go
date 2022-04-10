package main

import (
	"context"
	"strconv"
	"sync"
)

type worker struct {
	cancelPrev func()
	cancelCurr func()
	dbg        func(string)
	wg         *sync.WaitGroup
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

func newWorker(wg *sync.WaitGroup, dbg func(string)) *worker {
	return &worker{
		cancelPrev: func() {},
		cancelCurr: func() {},
		dbg:        dbg,
		wg:         wg,
	}
}

type extractorWorker struct {
	worker
	p *playlist
}

func (w *extractorWorker) run(link string) {
	ctx, cancel := context.WithCancel(context.Background())
	w.cancelPrevJob(cancel)

	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		items, err := processmediapage(ctx, link, w.dbg,
			func(str string) {
				msg := newMessage(textMessage, str)
				w.wg.Add(1)
				go func() {
					defer w.wg.Done()
					text <- msg
				}()
			})
		if err != nil {
			text <- newMessage(errorMessage, err.Error())
			return
		}

		err = w.p.Add(items)
		if err != nil {
			text <- newMessage(errorMessage, err.Error())
		}
	}()
}

func newExtractor(wg *sync.WaitGroup, p *playlist, dbg func(string)) *extractorWorker {
	return &extractorWorker{
		worker: *newWorker(wg, dbg),
		p:      p,
	}
}

type downloadWorker struct {
	worker
	cache *FIFO
}

func (w *downloadWorker) run(link string, n int) {
	ctx, cancel := context.WithCancel(context.Background())
	w.cancelPrevJob(cancel)

	prefix := "track " + strconv.Itoa(n+1) + " "

	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		result, err := downloadmedia(ctx, link, w.dbg,
			func(str string) {
				msg := newMessage(textMessage, str)
				msg.prefix = prefix
				// do not wait for other end, send and forget
				w.wg.Add(1)
				go func() {
					defer w.wg.Done()
					text <- msg
				}()
			})
		if err != nil {
			text <- newMessage(errorMessage, prefix+err.Error())
		} else {
			key := getTruncatedURL(link)
			if key == "" {
				text <- newMessage(errorMessage, "incorrect cache key")
				return
			}
			w.cache.Set(key, result)
		}
	}()
}

func newDownloader(wg *sync.WaitGroup, cache *FIFO, dbg func(string)) *downloadWorker {
	return &downloadWorker{
		worker: *newWorker(wg, dbg),
		cache:  cache,
	}
}
