package main

import (
	"fmt"
	"math/rand"
	"strings"
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
	asciionly bool

	artDrawingMode int
	artM           *artModel
	playerM        *playerModel
}

func (window *windowLayout) sendEvent(data interface{}) {
	window.HandleEvent(tcell.NewEventInterrupt(data))
}

func getNewTrack(track int) {
	if url, streamable := window.playerM.getURL(track); streamable {
		go downloadMedia(url, track)
	} else {
		newMessage(
			fmt.Sprintf("track %d is not available for streaming", track+1),
		)
		player.status = stopped
	}
}

func (window *windowLayout) HandleEvent(event tcell.Event) bool {
	switch event := event.(type) {

	case *tcell.EventInterrupt:
		switch data := event.Data().(type) {

		case *eventNewItem:
			if data.value() != nil {
				player.stop()
				player.clear()
				player.initPlayer()
				window.playerM.metadata = data.value()
				getNewTrack(player.currentTrack)
				go downloadCover(window.playerM.getImageURL(3))
				player.totalTracks = data.value().totalTracks
			}

		case *eventNextTrack:
			getNewTrack(data.value())

		case *eventTrackDownloaded:
			track := player.currentTrack
			if data.value() == window.playerM.getCacheID(track) &&
				!player.isPlaying() {

				player.play(track, data.value())
				return true
			}
			return false
		}
	case *tcell.EventKey:
		switch event.Key() {

		case tcell.KeyEscape:
			app.Quit()
			logFile.WriteString(event.When().Format(time.ANSIC) + "[ext]:exit with code 0\n")
			return true

		case tcell.KeyF5:
			// TODO: remove, doesn't do anything really
			app.Refresh()
			return true

		case tcell.KeyRune:
			if window.hideInput {
				switch event.Rune() {

				case 'i', 'I':
					window.artDrawingMode = (window.artDrawingMode + 1) % 6
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
			// TODO: remove these two later, for debug
		case tcell.KeyCtrlD:
			newDebugMessage(fmt.Sprint(window.playerM.metadata))
			return true

		case tcell.KeyCtrlT:
			for _, widget := range window.widgets {
				widget.(recolorable).SetStyle(getRandomStyle())
				window.artM.style = getRandomStyle()
			}
			return true
		}
	}
	return window.BoxLayout.HandleEvent(event)
}

// FIXME ?
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

type contentArea struct {
	views.Text
}

func (content *contentArea) HandleEvent(event tcell.Event) bool {
	switch event := event.(type) {

	case *views.EventWidgetResize:
		if window.hasChangedSize() {
			window.checkOrientation()
			return true
		}

	case *tcell.EventInterrupt:
		if event.Data() == nil {
			content.SetText(window.playerM.updateText())
			app.Update()
			return true
		}
		switch event.Data().(type) {

		case eventNewItem:
			window.playerM.updateText()
			return true
		}
		return false
	}
	return content.Text.HandleEvent(event)
}

// TODO: change back later
func (content *contentArea) Size() (int, int) {
	if window.orientation == views.Horizontal {
		window.playerM.endx = window.width - window.artM.endx - 3*window.hMargin
		window.playerM.endy = window.height - window.vMargin - 2
	} else {
		window.playerM.endx = window.width - 2*window.vMargin
		window.playerM.endy = window.height - 2*window.vMargin - window.artM.endy - 2
	}
	return window.playerM.endx, window.playerM.endy
}

type playerModel struct {
	metadata *album
	endx     int
	endy     int
}

func (model *playerModel) getURL(track int) (string, bool) {
	if model.metadata.tracks[track].url != "" {
		return model.metadata.tracks[track].url, true
	} else {
		return "", false
	}
}

func (model *playerModel) getCacheID(track int) string {
	return getTruncatedURL(model.metadata.tracks[track].url)
}

// a<album_art_id>_nn.jpg
// other images stored without type prefix?
// not all sizes are listed here, all up to _16 are existing files
// _10 - original, whatever size it was
// _16 - 700x700
// _7  - 160x160
// _3  - 100x100
func (model *playerModel) getImageURL(size int) string {
	var s string
	switch size {
	case 3:
		s = "_16"
	case 2:
		s = "_7"
	case 1:
		s = "_3"
	default:
		return model.metadata.imageSrc
	}
	return strings.Replace(model.metadata.imageSrc, "_10", s, 1)
}

func (model *playerModel) updateText() string {
	var volume string
	var repeats int
	timeStamp := player.getCurrentTrackPosition()
	track := player.currentTrack

	if model.metadata == nil {
		model.metadata = getDummyData()
		// FIXME: not a good place for this
		player.totalTracks = 0
	}

	duration := model.metadata.tracks[track].duration
	if duration > 0 {
		repeats = int((float64(timeStamp) / (duration * 1_000_000_000)) * float64(model.endx))
	} else {
		repeats = 0
	}

	if player.muted {
		volume = "mute"
	} else {
		volume = fmt.Sprintf("%3.0f", (100 + player.volume*10))
	}

	var symbol string
	if window.asciionly {
		symbol = "="
	} else {
		symbol = "\u25b1"
	}

	return fmt.Sprintf(model.metadata.formatString(track),
		player.status.String(),
		strings.Repeat(symbol, repeats),
		timeStamp,
		volume,
		player.playbackMode.String(),
	)
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

	case *tcell.EventKey:
		if event.Key() == tcell.KeyTab {
			if !window.hideInput {
				message.SetText("press [Tab] to enable input")
			} else {
				message.SetText("enter link/command")
			}
		}

	case *tcell.EventInterrupt:
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
		}
		return false
	}
	return message.Text.HandleEvent(event)
}

func (message *messageBox) Size() (int, int) {
	if window.orientation == views.Horizontal {
		return window.width - window.artM.endx - 3*window.hMargin, 1
	}
	return window.width - 2*window.vMargin, 1
}

func getRandomStyle() tcell.Style {
	rand.Seed(time.Now().UnixNano())
	return tcell.StyleDefault.Foreground(
		tcell.NewHexColor(
			int32(rand.Intn(2_147_483_647)))).Background(
		tcell.NewHexColor(
			int32(rand.Intn(2_147_483_647))))
}

func changeTheme() {
	window.theme = (window.theme + 1) % 3

	var style tcell.Style
	switch window.theme {

	case 1, 2:
		window.fgColor, window.bgColor = window.bgColor, window.fgColor
		style = tcell.StyleDefault.Background(window.bgColor).
			Foreground(window.fgColor)

	default:
		style = tcell.StyleDefault
	}
	for i, widget := range window.widgets {
		if i == 5 && window.theme != 0 {
			widget.(recolorable).SetStyle(style.Foreground(tcell.ColorLightSlateGray))
			continue
		}
		widget.(recolorable).SetStyle(style)
	}
	window.artM.style = style
}

func init() {
	var err error
	window.hideInput = true
	window.hMargin, window.vMargin = 3, 1
	window.fgColor = tcell.NewHexColor(0xf9fdff)
	window.bgColor = tcell.NewHexColor(0x2b2b2b)

	window.playerM = &playerModel{}

	spacer1 := &spacer{views.NewText(), false}
	art := &artArea{views.NewCellView()}
	art.SetModel(window.artM)
	spacer2 := &spacer{views.NewText(), false}
	contentBoxH := views.NewBoxLayout(views.Horizontal)
	contentBoxV1 := views.NewBoxLayout(views.Vertical)
	contentBoxV2 := views.NewBoxLayout(views.Vertical)
	spacer3 := &spacer{views.NewText(), true}
	content := &contentArea{}
	content.SetText(window.playerM.updateText())
	message := &messageBox{views.NewText()}
	message.SetText("press [Tab] to enable input")
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
	window.screen, err = tcell.NewScreen()
	checkFatalError(err)
	app.SetScreen(window.screen)
	app.SetRootWidget(window)
}
