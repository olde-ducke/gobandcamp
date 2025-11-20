package main

import (
	"errors"
	"io"
	"sync"
)

type callback struct {
	io.ReadSeeker
	once sync.Once
	f    func()
}

func newCallback(reader io.Reader, f func()) *callback {
	r, ok := reader.(io.ReadSeeker)
	if !ok {
		panic("provided io.Reader must implement io.Seeker")
	}

	return &callback{r, sync.Once{}, f}
}

func (c *callback) Read(p []byte) (int, error) {
	n, err := c.ReadSeeker.Read(p)
	if errors.Is(err, io.EOF) {
		c.once.Do(c.f)
	}

	return n, err
}

type length interface {
	Length() int64
}
