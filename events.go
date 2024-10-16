package main

import (
	"image"

	"github.com/gdamore/tcell/v2"
)

type eventUpdate struct {
	tcell.EventTime
}

type eventDisplayMessage struct {
	tcell.EventTime
}

type eventRefitArt struct {
	tcell.EventTime
}

type eventCheckDrawMode struct {
	tcell.EventTime
}

type textEvents interface {
	String() string
}

// new media item data, if null, previous playback will continue
type eventNewItem struct {
	tcell.EventTime
	album *album
}

func newItem(album *album) *eventNewItem {
	return &eventNewItem{album: album}
}

func (event *eventNewItem) value() *album {
	return event.album
}

type eventNewTagSearch struct {
	tcell.EventTime
	result *DiscoverResult
}

func newTagSearch(result *DiscoverResult) *eventNewTagSearch {
	return &eventNewTagSearch{result: result}
}

func (event *eventNewTagSearch) value() *DiscoverResult {
	return event.result
}

type eventAdditionalTagSearch struct {
	tcell.EventTime
	result *DiscoverResult
}

func newAdditionalTagSearch(result *DiscoverResult) *eventAdditionalTagSearch {
	return &eventAdditionalTagSearch{result: result}
}

func (event *eventAdditionalTagSearch) value() *DiscoverResult {
	return event.result
}

type eventNextTrack struct {
	tcell.EventTime
}

// value unused
type eventNewTrack struct {
	tcell.EventTime
	track int
}

func newTrack(trackNumber int) *eventNewTrack {
	return &eventNewTrack{track: trackNumber}
}

func (event *eventNewTrack) value() int {
	return event.track
}

type eventCoverDownloaded struct {
	tcell.EventTime
	cover image.Image
	url   string
}

func newCoverDownloaded(cover image.Image, url string) *eventCoverDownloaded {
	return &eventCoverDownloaded{cover: cover, url: url}
}

func (event *eventCoverDownloaded) value() image.Image {
	return event.cover
}

func (event *eventCoverDownloaded) key() string {
	return event.url
}

// cache key = track url
type eventTrackDownloaded struct {
	tcell.EventTime
	key string
}

func newTrackDownloaded(key string) *eventTrackDownloaded {
	return &eventTrackDownloaded{key: key}
}

func (event *eventTrackDownloaded) value() string {
	return event.key
}

type eventDebugMessage struct {
	tcell.EventTime
	message string
}

func newDebugMessage(text string) *eventDebugMessage {
	var event tcell.EventTime
	event.SetEventNow()
	return &eventDebugMessage{event, text}
}

func (event *eventDebugMessage) String() string {
	return event.message
}

type eventMessage struct {
	tcell.EventTime
	message string
}

func newMessage(text string) *eventMessage {
	var event tcell.EventTime
	event.SetEventNow()
	return &eventMessage{event, text}
}

func (event *eventMessage) String() string {
	return event.message
}

type eventErrorMessage struct {
	tcell.EventTime
	message error
}

func newErrorMessage(err error) *eventErrorMessage {
	var event tcell.EventTime
	event.SetEventNow()
	return &eventErrorMessage{event, err}
}

func (event *eventErrorMessage) String() string {
	return event.message.Error()
}
