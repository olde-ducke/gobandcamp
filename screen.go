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
var exitCode = 0

type eventNewItem int
type eventNextTrack int
type eventCoverDownloader int
type eventTrackDownloader int
type eventDebugMessage string

/*func newDebugMessage(message string) eventDebugMessage {
	return eventDebugMessage(message)
}*/

func (message *eventDebugMessage) String() string {
	return string(*message)
}

type recolorable interface {
	SetStyle(tcell.Style)
}

type windowLayout struct {
	views.BoxLayout
	screen         tcell.Screen
	hideInput      bool
	width          int
	height         int
	artM           *artModel
	playerM        *playerModel
	artDrawingMode int
	orientation    int
	hMargin        int
	vMargin        int
	widgets        []views.Widget
	bgColor        tcell.Color
	fgColor        tcell.Color
	theme          int
}

func (window *windowLayout) sendPlayerEvent(data interface{}) {
	window.HandleEvent(tcell.NewEventInterrupt(data))
}

func (window *windowLayout) HandleEvent(event tcell.Event) bool {
	switch event := event.(type) {
	case *tcell.EventInterrupt:
		switch data := event.Data().(type) {
		// TODO: isn't it silly to send an empty link and check if it's empty only
		// on other side?
		case eventNewItem:
			if data >= 0 {
				go downloadMedia(window.playerM.metadata.tracks[0].url)
				go downloadCover(window.playerM.metadata.imageSrc, window.artM)
				player.totalTracks = window.playerM.metadata.totalTracks
			}
		case eventNextTrack:
			go downloadMedia(window.playerM.metadata.tracks[data].url)
		case eventTrackDownloader:
			if data == eventTrackDownloader(player.currentTrack) && !player.isPlaying() {
				player.play(int(data))
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
		case tcell.KeyTab:
			window.hideInput = !window.hideInput
		case tcell.KeyF5:
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
				default:
					return player.handleEvent(event.Rune())
				}
			}
		case tcell.KeyCtrlC:
			window.screen.Fini()
			return true
		case tcell.KeyCtrlD:
			for _, widget := range window.widgets {
				// FIXME: might fail after adding new widgets without
				// this method
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
		switch data := event.Data().(type) {
		case eventNewItem:
			if data < 0 {
				window.playerM.metadata = nil
			}
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
	dummy    *album
	endx     int
	endy     int
}

func (model *playerModel) updateText() string {
	var formatString string
	var volume string
	var repeats int
	timeStamp := player.getCurrentTrackPosition()

	if model.metadata == nil {
		formatString = model.dummy.formatString(player.currentTrack)
		repeats = 0
	} else {
		formatString = model.metadata.formatString(player.currentTrack)
		duration := model.metadata.tracks[player.currentTrack].duration
		if duration > 0 {
			repeats = int((float64(timeStamp) / (duration * 1_000_000_000)) * float64(model.endx))
		} else {
			repeats = 0
		}
	}

	if player.muted {
		volume = "mute"
	} else {
		volume = fmt.Sprintf("%3.0f", (100 + player.volume*10))
	}

	return fmt.Sprintf(formatString,
		player.status.String(),
		strings.Repeat("\u25b1", repeats),
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
	case *tcell.EventInterrupt:
		switch data := event.Data().(type) {
		case string:
			logFile.WriteString(event.When().Format(time.ANSIC) + "[msg]:" + data + "\n")
			message.SetText(data)
			return true
		case error:
			logFile.WriteString(event.When().Format(time.ANSIC) + "[err]:" + data.Error() + "\n")
			message.SetText(data.Error())
			return true
		case eventDebugMessage:
			logFile.WriteString(event.When().Format(time.ANSIC) + "[dbg]:" + data.String() + "\n")
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
	case 1:
		style = tcell.StyleDefault.Background(window.bgColor).
			Foreground(window.fgColor)
	case 2:
		style = tcell.StyleDefault.Background(window.fgColor).
			Foreground(window.bgColor)
	default:
		style = tcell.StyleDefault
	}
	for _, widget := range window.widgets {
		widget.(recolorable).SetStyle(style)
		window.artM.style = style
	}
}

func init() {
	var err error
	window.hideInput = true
	window.hMargin, window.vMargin = 3, 1
	window.fgColor = tcell.NewHexColor(0xf9fdff)
	window.bgColor = tcell.NewHexColor(0x2b2b2b)

	/*window.artM = &artModel{}
	window.artM.placeholder, err = png.Decode(bytes.NewReader(gopherPNG))
	if err != nil {
		checkFatalError(err)
	}
	window.artM.converter = *convert.NewImageConverter()*/

	window.playerM = &playerModel{}
	window.playerM.dummy = getDummyData()

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
	message.SetText("press Tab and enter album link:")
	field := &textField{}
	field.EnableCursor(!window.hideInput)
	field.HideCursor(window.hideInput)
	field.Clear()
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
	// FIXME: messy
	window.widgets = []views.Widget{spacer1, art, spacer2, spacer3, content, message, field, spacer4}

	// create new screen to gain access to actual terminal dimensions
	window.screen, err = tcell.NewScreen()
	checkFatalError(err)
	app.SetScreen(window.screen)
	app.SetRootWidget(window)

	// sometimes doesn't work inside main event loop?
	//app.PostFunc(func() {
	go func() {
		for {
			time.Sleep(time.Second / 2)
			window.sendPlayerEvent(nil)
			if player.status == seekBWD || player.status == seekFWD {
				player.status = player.bufferedStatus
			}
		}
	}()
	//})
}
