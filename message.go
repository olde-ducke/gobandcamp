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

	case *tcell.EventInterrupt:
		if *debug {
			switch data := event.Data().(type) {

			case *eventMessage:
				logFile.WriteString(event.When().Format(time.ANSIC) + "[msg]:" + data.string() + "\n")
				message.SetText(data.string())
				return true

			case *eventErrorMessage:
				logFile.WriteString(event.When().Format(time.ANSIC) + "[err]:" + data.string() + "\n")
				message.SetText(data.string())
				return true

			case *eventDebugMessage:
				logFile.WriteString(event.When().Format(time.ANSIC) + "[dbg]:" + data.string() + "\n")
				return true
			}
		} else {
			switch data := event.Data().(type) {

			case textEvents:
				message.SetText(data.string())
				return true
			}
		}
	}
	return false //message.Text.HandleEvent(event)
}

func (message *messageBox) Size() (int, int) {
	width, _ := window.getBounds()
	return width, 1
}

// TODO: remove if message ever becomes some other widget?
func (message *messageBox) SetStyle(style tcell.Style) {
	message.Text.SetStyle(style.Foreground(window.altColor))
}

func init() {
	messageBox := &messageBox{views.NewText()}
	messageBox.SetText("[Tab] enable input [H] display help")
	window.widgets[message] = messageBox
}
