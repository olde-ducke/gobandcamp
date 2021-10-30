package main

import (
	"image"
)

type textEvents interface {
	string() string
}

// new media item data, if null, previous playback will continue
type eventNewItem struct {
	album *album
}

func newItem(album *album) *eventNewItem {
	return &eventNewItem{album}
}

func (event *eventNewItem) value() *album {
	return event.album
}

// value unused
type eventNextTrack struct {
	track int
}

func newTrack(trackNumber int) *eventNextTrack {
	return &eventNextTrack{trackNumber}
}

func (event *eventNextTrack) value() int {
	return event.track
}

type eventCoverDownloaded struct {
	cover image.Image
}

func newCoverDownloaded(cover image.Image) *eventCoverDownloaded {
	return &eventCoverDownloaded{cover}
}

func (event *eventCoverDownloaded) value() image.Image {
	return event.cover
}

// cache key = track url
type eventTrackDownloaded struct {
	key string
}

func newTrackDownloaded(key string) *eventTrackDownloaded {
	return &eventTrackDownloaded{key}
}

func (event *eventTrackDownloaded) value() string {
	return event.key
}

// simple wrapper for string, debug mesages are not displayed on screen,
// unlike error or info mesages, which use default error and string types
type eventDebugMessage struct {
	message string
}

func newDebugMessage(text string) *eventDebugMessage {
	return &eventDebugMessage{text}
}

func (event *eventDebugMessage) string() string {
	return event.message
}

type eventMessage struct {
	message string
}

func newMessage(text string) *eventMessage {
	return &eventMessage{text}
}

func (event *eventMessage) string() string {
	return event.message
}

type eventErrorMessage struct {
	message error
}

func newErrorMessage(err error) *eventErrorMessage {
	return &eventErrorMessage{err}
}

func (event *eventErrorMessage) string() string {
	return event.message.Error()
}
