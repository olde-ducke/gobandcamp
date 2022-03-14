package main

import (
	"container/list"
	"sync"
)

type FIFO struct {
	sync.RWMutex
	cache map[string][]byte
	queue *list.List
	size  int
}

func NewCache(size int) *FIFO {
	return &FIFO{
		cache: make(map[string][]byte, size),
		queue: list.New(),
		size:  size,
	}
}

func (fifo *FIFO) Set(key string, value []byte) {
	fifo.Lock()
	defer fifo.Unlock()
	if _, ok := fifo.cache[key]; ok {
		return
	}

	if len(fifo.cache) < fifo.size {
		fifo.cache[key] = value
		fifo.queue.PushBack(key)
		return
	}

	enqueue := fifo.queue.Front()
	delete(fifo.cache, enqueue.Value.(string))
	fifo.queue.Remove(enqueue)
	fifo.cache[key] = value
	fifo.queue.PushBack(key)
}

func (fifo *FIFO) Get(key string) ([]byte, bool) {
	fifo.Lock()
	value, ok := fifo.cache[key]
	fifo.Unlock()
	return value, ok
}

func (fifo *FIFO) Dump() []string {
	var d []string
	enqueue := fifo.queue.Front()
	for i := 0; i < fifo.size; i++ {
		var value interface{}
		value = ""
		if enqueue != nil {
			value = enqueue.Value
			enqueue = enqueue.Next()
		}
		d = append(d, value.(string))
	}
	return d
}
