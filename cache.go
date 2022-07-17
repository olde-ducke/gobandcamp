package main

import (
	"container/list"
	"sync"
)

type Storage interface {
	Set(string, any)
	Get(string) (any, bool)
	Dump() []string
}

// FIFO simple first in first out cache
type FIFO struct {
	sync.RWMutex
	cache map[string]any
	queue *list.List
	size  int
}

// NewCache returns simple FIFO cache with given size
func NewCache(size int) *FIFO {
	return &FIFO{
		cache: make(map[string]any, size),
		queue: list.New(),
		size:  size,
	}
}

// Set stores data to cache.
func (fifo *FIFO) Set(key string, value any) {
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
func (fifo *FIFO) Get(key string) (any, bool) {
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
		var value any
		value = ""
		if enqueue != nil {
			value = enqueue.Value
			enqueue = enqueue.Next()
		}
		dump[i] = value.(string)
	}
	return dump
}
