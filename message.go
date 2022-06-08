package main

import (
	"fmt"
	"sync"
	"time"
)

const formatString = "%s [%s]: %s%s\n"

type messageType int

const (
	debugMessage messageType = iota
	errorMessage
	textMessage
	infoMessage
)

var types = [4]string{"dbg", "err", "msg", "inf"}

func (t messageType) String() string {
	return types[t]
}

type message struct {
	msgType   messageType
	timestamp time.Time
	prefix    string
	text      string
}

func (msg *message) When() time.Time {
	return msg.timestamp
}

func (msg *message) Type() messageType {
	return msg.msgType
}

func (msg *message) String() string {
	return fmt.Sprintf(formatString, msg.timestamp.Format(time.ANSIC),
		msg.msgType, msg.prefix, msg.text)
}

func (msg *message) Text() string {
	return msg.prefix + msg.text
}

func newMessage(t messageType, prefix, format string, args ...any) *message {
	return &message{
		msgType:   t,
		prefix:    prefix,
		text:      fmt.Sprintf(format, args...),
		timestamp: time.Now(),
	}
}

func newReporter(t messageType, prefix string, wg *sync.WaitGroup, out chan<- *message) func(string, ...any) {
	return func(format string, args ...any) {
		msg := newMessage(t, prefix, format, args...)
		wg.Add(1)
		go func() {
			defer wg.Done()
			out <- msg
		}()
	}
}
