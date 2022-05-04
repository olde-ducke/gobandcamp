package main

import (
	"errors"
	"strings"
)

type fileManager struct {
	music *FIFO
	do    chan<- *action
}

func (fm *fileManager) open(path string) error {
	if strings.HasPrefix(path, "file://") {
		return errors.New("NOT IMPLEMENTED: local file reading")
	}

	if _, ok := fm.music.Get(getTruncatedURL(path)); ok {
		fm.do <- &action{actionPlay, path}
		return nil
	}

	fm.do <- &action{actionDownload, path}
	return errors.New("data not found, downloading")
}

func newFileManager(cache *FIFO, do chan<- *action) *fileManager {
	return &fileManager{cache, do}
}
