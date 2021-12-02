package main

import (
	"bytes"
	"container/list"
	"sync"
)

type FIFO struct {
	sync.RWMutex
	cache map[interface{}][]byte
	queue *list.List
	size  int
}

func newCache(size int) *FIFO {
	return &FIFO{
		cache: make(map[interface{}][]byte, size),
		queue: list.New(),
		size:  size,
	}
}

func (fifo *FIFO) set(key interface{}, value []byte) {
	fifo.Lock()
	defer fifo.Unlock()
	//defer fifo.dump()
	if _, ok := fifo.cache[key]; ok {
		return
	}

	if len(fifo.cache) < fifo.size {
		fifo.cache[key] = value
		fifo.queue.PushBack(key)
		return
	}

	enqueue := fifo.queue.Front()
	delete(fifo.cache, enqueue.Value)
	fifo.queue.Remove(enqueue)
	fifo.cache[key] = value
	fifo.queue.PushBack(key)
}

func (fifo *FIFO) get(key string) ([]byte, bool) {
	fifo.Lock()
	//fifo.dump()
	value, ok := fifo.cache[key]
	fifo.Unlock()
	return value, ok
}

/*
func (fifo *FIFO) dump() {
	enqueue := fifo.queue.Front()
	for i := 0; i < fifo.size; i++ {
		var value interface{}
		value = ""
		if enqueue != nil {
			value = enqueue.Value
			enqueue = enqueue.Next()
		}
		window.sendEvent(newDebugMessage(value.(string)))
	}
}
*/

// response.Body doesn't implement Seek() method
// beep isn't bothered by this, but trying to
// call Seek() will fail since Len() will always return 0
// by using bytes.Reader and implementing empty Close() method
// we get io.ReadSeekCloser, which satisfies requirements of beep streamers
// (need ReadCloser) and implements Seek() method
// TODO: remove later

type bytesReadSeekCloser struct {
	*bytes.Reader
}

func (c bytesReadSeekCloser) Close() error {
	return nil
}

func wrapInRSC(key string) *bytesReadSeekCloser {
	value, ok := cache.get(key)
	if !ok {
		return &bytesReadSeekCloser{bytes.NewReader([]byte{0})}
	}
	return &bytesReadSeekCloser{bytes.NewReader(value)}
}
