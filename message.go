package main

import (
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/gdamore/tcell/v2/views"
)

type messageBox struct {
	*views.Text
}

func (message *messageBox) HandleEvent(event tcell.Event) bool {
	switch event := event.(type) {

	case *eventMessage:
		logFile.WriteString(event.When().Format(time.ANSIC) + "[msg]:" + event.String() + "\n")
		message.SetText(event.String())
		return true

	case *eventErrorMessage:
		logFile.WriteString(event.When().Format(time.ANSIC) + "[err]:" + event.String() + "\n")
		message.SetText(event.String())
		return true
	}
	return false //message.Text.HandleEvent(event)
}

func (message *messageBox) Size() (int, int) {
	width, _ := window.getBounds()
	return width, 1
}

// TODO: remove if message ever becomes some other widget?
func (message *messageBox) SetStyle(style tcell.Style) {
	message.Text.SetStyle(style.Foreground(window.accentColor))
}

func init() {
	messageBox := &messageBox{views.NewText()}
	messageBox.SetText("messages will show up here, press [Esc] to quit")
	window.widgets[message] = messageBox
}
