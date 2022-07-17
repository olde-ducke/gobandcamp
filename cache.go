package main

import (
	"container/list"
	"sync"
)

// FIFO simple first in first out cache
type FIFO struct {
	sync.RWMutex
	cache map[string]interface{}
	queue *list.List
	size  int
}

// NewCache returns simple FIFO cache with given size
func NewCache(size int) *FIFO {
	return &FIFO{
		cache: make(map[string]interface{}, size),
		queue: list.New(),
		size:  size,
	}
}

// Set stores data to cache.
func (fifo *FIFO) Set(key string, value interface{}) {
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

// Get returns data from cache.
func (fifo *FIFO) Get(key string) (interface{}, bool) {
	fifo.Lock()
	value, ok := fifo.cache[key]
	fifo.Unlock()
	return value, ok
}

// Dump returns all keys as slice.
func (fifo *FIFO) Dump() []string {
	dump := make([]string, fifo.size)
	enqueue := fifo.queue.Front()
	for i := 0; i < fifo.size; i++ {
		var value interface{}
		value = ""
		if enqueue != nil {
			value = enqueue.Value
			enqueue = enqueue.Next()
		}
		dump[i] = value.(string)
	}
	return dump
}

/*
// TODO: delete later and replace with SQLite?
type simpleCache struct {
	sync.RWMutex
	cache map[string][]item
	size  int
}

func newSimpleCache(size int) *simpleCache {
	return &simpleCache{
		cache: make(map[string][]item, size),
		size:  size,
	}
}

func (c *simpleCache) Set(key string, value []item) {
	c.Lock()
	c.cache[key] = value
	c.Unlock()
}

func (c *simpleCache) Get(key string) ([]item, bool) {
	c.Lock()
	value, ok := c.cache[key]
	c.Unlock()
	return value, ok
}

func (c *simpleCache) Dump() []string {
	var dump []string
	if size := len(c.cache); size > c.size {
		dump = make([]string, size)
	} else {
		dump = make([]string, c.size)
	}

	c.Lock()
	defer c.Unlock()

	var i int
	for k := range c.cache {
		dump[i] = k
		i++
	}

	return dump
}
*/
