package main

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/gdamore/tcell/v2/views"
)

var app = &views.Application{}
var window = &windowLayout{}

type recolorable interface {
	SetStyle(tcell.Style)
}

type windowLayout struct {
	views.BoxLayout
	screen tcell.Screen

	width       int
	height      int
	orientation int
	hMargin     int
	vMargin     int

	hideInput bool

	theme     int
	widgets   []views.Widget
	bgColor   tcell.Color
	fgColor   tcell.Color
	altColor  tcell.Color
	style     tcell.Style
	asciionly bool

	boundx, boundy int
	playlist       *album
	artM           *artModel
	playerM        *playerModel
	textM          *textModel
	playlistM      *model
	artDrawingMode int
}

func (window *windowLayout) sendEvent(data interface{}) {
	if _, ok := data.(*eventDebugMessage); ok && !*debug {
		return
	}
	window.HandleEvent(tcell.NewEventInterrupt(data))
}

func getNewTrack(track int) {
	if url, streamable := window.playlist.getURL(track); streamable {
		go downloadMedia(url, track)
	} else {
		window.sendEvent(newMessage("track is not available for streaming"))
		player.status = stopped
	}
}

func (window *windowLayout) HandleEvent(event tcell.Event) bool {
	switch event := event.(type) {

	case *tcell.EventInterrupt:
		switch data := event.Data().(type) {

		// FIXME: for now do not consume these two event types and
		// pass them to child widget, works? but tcell docs say you
		// SHOULD always either return true or false
		case *eventNewItem:
			if data.value() != nil {
				player.stop()
				player.clearStream()
				window.playlist = data.value()
				player.currentTrack = 0
				getNewTrack(player.currentTrack)
				go downloadCover(window.playlist.getImageURL(3))
				player.totalTracks = data.value().totalTracks
			}
			//return true

		case *eventNextTrack:
			//track := data.value()
			getNewTrack(data.value())
			//return true

		case *eventTrackDownloaded:
			track := player.currentTrack
			if data.value() == window.playlist.getCacheID(track) {
				if player.status == playing {
					player.stop()
					player.clearStream()
				}
				player.play(data.value())
				return true
			}
			return false
		}

	case *tcell.EventKey:
		switch event.Key() {

		case tcell.KeyEscape:
			app.Quit()
			return true

			// dumps all parsed metadata to logfile
		case tcell.KeyCtrlD:
			window.sendEvent(newDebugMessage(fmt.Sprint(window.playlist)))
			return true

		// recolor everything in random colors
		// if debug flag is not set everything in one random style
		case tcell.KeyCtrlT:
			window.altColor = getRandomColor()
			if *debug {
				for _, widget := range window.widgets {
					widget.(recolorable).SetStyle(getRandomStyle())
				}
				window.style = getRandomStyle()
			} else {
				window.style = getRandomStyle()
				for i, widget := range window.widgets {
					if i == 5 {
						widget.(recolorable).SetStyle(window.style.Foreground(window.altColor))
					} else {
						widget.(recolorable).SetStyle(window.style)
					}
				}
			}
			return true

		case tcell.KeyRune:
			if window.hideInput {
				switch event.Rune() {

				case 'i', 'I':
					window.artDrawingMode = (window.artDrawingMode + 1) % 6
					checkDrawingMode()
					return true

				case 't', 'T':
					changeTheme()
					return true

				case 'e', 'E':
					window.asciionly = !window.asciionly
					return true

				default:
					return player.handleEvent(event.Rune())
				}
			}
		}
	}
	return window.BoxLayout.HandleEvent(event)
}

// FIXME: this assumes that font is 1/2 height to width
func (window *windowLayout) checkOrientation() {
	if window.width > 2*window.height {
		window.SetOrientation(views.Horizontal)
		window.orientation = views.Horizontal
	} else {
		window.SetOrientation(views.Vertical)
		window.orientation = views.Vertical
	}
}

// FIXME: on the start ~64 resize events are captured, every actual
// screen resize captures 36, evey key press captures 14
// don't know what's happening here, this prevents unnecessary
// redraws and image conversion
func (window *windowLayout) hasChangedSize() bool {
	width, height := window.screen.Size()
	if window.width != width || window.height != height {
		window.width = width
		window.height = height
		return true
	}
	return false
}

func (window *windowLayout) recalculateBounds() {

	if window.orientation == views.Horizontal {
		window.boundx = window.width - window.artM.endx - 3*window.hMargin
		window.boundy = window.height - window.vMargin - 2
	} else {
		window.boundx = window.width - 2*window.vMargin
		window.boundy = window.height - 2*window.vMargin - window.artM.endy - 2
	}

	// clamp to zero, otherwise can lead to negative indices
	if window.boundx < 0 {
		window.boundx = 0
	}

	if window.boundy < 0 {
		window.boundy = 0
	}
}

func (window *windowLayout) getBounds() (int, int) {
	return window.boundx, window.boundy
}

type spacer struct {
	*views.Text
	dynamic bool
}

func (s *spacer) Size() (int, int) {
	if s.dynamic && window.orientation != views.Horizontal {
		return 1, 1
	}
	return window.hMargin, window.vMargin
}

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
	if window.orientation == views.Horizontal {
		return window.width - window.artM.endx - 3*window.hMargin, 1
	}
	return window.width - 2*window.vMargin, 1
}

func getRandomStyle() tcell.Style {
	return tcell.StyleDefault.Foreground(
		getRandomColor()).Background(getRandomColor())
}

func getRandomColor() tcell.Color {
	rand.Seed(time.Now().UnixNano())
	return tcell.NewHexColor(
		int32(rand.Intn(2_147_483_647)))
}

func changeTheme() {
	window.theme = (window.theme + 1) % 3

	switch window.theme {

	case 1, 2:
		window.fgColor, window.bgColor = window.bgColor, window.fgColor
		window.style = tcell.StyleDefault.Background(window.bgColor).
			Foreground(window.fgColor)
		window.altColor = tcell.ColorLightSlateGray

	default:
		window.style = tcell.StyleDefault
		window.altColor = 0
	}

	for i, widget := range window.widgets {
		// TODO: delete later
		if i == 5 && window.theme != 0 {
			widget.(recolorable).SetStyle(window.style.Foreground(window.altColor))
			continue
		}
		widget.(recolorable).SetStyle(window.style)
	}
	checkDrawingMode()
}

func checkDrawingMode() {
	// if light theme and colored symbols on background color drawing mode
	// selected, reverse color drawing option (by defauplayerult black is basically
	// treated as transparent) and redraw image, if any other mode selected
	// and reversing is still enabled, reverse to default and redraw,
	// looks bad on white either way, but at least is more recognisable
	if window.theme == 1 && window.artDrawingMode == 5 {
		if !window.artM.options.Reversed {
			window.artM.options.Reversed = true
			window.artM.refitArt()
		}
	} else if window.artM.options.Reversed {
		window.artM.options.Reversed = false
		window.artM.refitArt()
	}
}

func init() {
	var err error
	window.hideInput = true
	window.hMargin, window.vMargin = 3, 1
	window.fgColor = tcell.NewHexColor(0xf9fdff)
	window.bgColor = tcell.NewHexColor(0x2b2b2b)

	window.playerM = &playerModel{}
	window.textM = &textModel{}
	window.playlistM = &model{enab: true, hide: true}

	spacer1 := &spacer{views.NewText(), false}
	art := &artArea{views.NewCellView()}
	art.SetModel(window.artM)
	spacer2 := &spacer{views.NewText(), false}
	contentBoxH := views.NewBoxLayout(views.Horizontal)
	contentBoxV1 := views.NewBoxLayout(views.Vertical)
	contentBoxV2 := views.NewBoxLayout(views.Vertical)
	spacer3 := &spacer{views.NewText(), true}
	content := &contentArea{views.NewCellView(), 0}
	content.SetModel(window.playerM)
	// TODO: clean up this mess
	//window.playerM.updateModel()
	message := &messageBox{views.NewText()}
	message.SetText("[Tab] enable input [H] display help")
	field := &textField{}
	field.EnableCursor(!window.hideInput)
	field.HideCursor(window.hideInput)
	field.Clear()
	field.previous = make([]rune, 1)
	field.previous[0] = ' '
	spacer4 := &spacer{views.NewText(), true}

	window.AddWidget(spacer1, 0.0)
	window.AddWidget(art, 0.0)
	contentBoxH.AddWidget(spacer3, 0.0)
	contentBoxV2.AddWidget(content, 1.0)
	contentBoxV2.AddWidget(message, 0.0)
	contentBoxV2.AddWidget(field, 0.0)
	contentBoxH.AddWidget(contentBoxV2, 0.0)
	contentBoxH.AddWidget(spacer4, 0.0)
	contentBoxV1.AddWidget(spacer2, 0.0)
	contentBoxV1.AddWidget(contentBoxH, 0.0)
	window.AddWidget(contentBoxV1, 1.0)

	window.widgets = []views.Widget{spacer1, art, spacer2, spacer3, content,
		message, field, spacer4}

	// create new screen to gain access to actual terminal dimensions
	// works on unix and windows, unlike ascii2image dependency
	window.screen, err = tcell.NewScreen()
	checkFatalError(err)
	app.SetScreen(window.screen)
	app.SetRootWidget(window)
}
