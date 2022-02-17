package main

import (
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
	defer fifo.dump()
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
	fifo.dump()
	value, ok := fifo.cache[key]
	fifo.Unlock()
	return value, ok
}

func (fifo *FIFO) dump() {
	enqueue := fifo.queue.Front()
	for i := 0; i < fifo.size; i++ {
		var value interface{}
		value = ""
		if enqueue != nil {
			value = enqueue.Value
			enqueue = enqueue.Next()
		}
		dbg("cache: " + value.(string))
	}
}
