package main

import (
	"bytes"
	_ "embed"
	"fmt"
	"image"
	"image/png"
	"math/rand"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/gdamore/tcell/v2/views"
	"github.com/qeesung/image2ascii/ascii"
	"github.com/qeesung/image2ascii/convert"
)

//go:embed assets/gopher.png
var gopherPNG []byte

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

type textField struct {
	views.TextArea
	symbols  []rune
	sbuilder strings.Builder
}

func (field *textField) HandleEvent(event tcell.Event) bool {
	// FIXME: jank
	field.HideCursor(window.hideInput)
	field.EnableCursor(!window.hideInput)
	if window.hideInput {
		return true
	}
	field.MakeCursorVisible()
	posX, _, _, _ := field.GetModel().GetCursor()

	switch event := event.(type) {

	case *tcell.EventKey:
		switch event.Key() {

		case tcell.KeyEnter:
			parseInput(field.getText())
			field.Clear()
			window.hideInput = !window.hideInput
			field.HideCursor(window.hideInput)
			field.EnableCursor(!window.hideInput)
			return true

		case tcell.KeyBackspace2:
			if posX > 0 {
				posX--
				field.symbols[posX] = 0
				field.symbols = append(field.symbols[:posX],
					field.symbols[posX+1:]...)
			}
			field.SetContent(string(field.symbols))
			field.SetCursorX(posX)
			return true

		case tcell.KeyDelete:
			if posX < len(field.symbols)-1 {
				field.symbols[posX] = 0
				field.symbols = append(field.symbols[:posX],
					field.symbols[posX+1:]...)
				posX++
			}
			field.SetContent(string(field.symbols))
			return true

		case tcell.KeyRune:
			field.symbols = append(field.symbols, 0)
			copy(field.symbols[posX+1:], field.symbols[posX:])
			field.symbols[posX] = event.Rune()
			field.SetContent(string(field.symbols))
			field.SetCursorX(posX + 1)
			return true
		}
	}
	return field.TextArea.HandleEvent(event)
}

func (field *textField) getText() string {
	for i, r := range field.symbols {
		// trailing space doesn't need to be in actual input
		if i == len(field.symbols)-1 {
			break
		}
		fmt.Fprint(&field.sbuilder, string(r))
	}
	defer field.sbuilder.Reset()

	return field.sbuilder.String()
}

func (field *textField) Clear() {
	field.SetContent(" ")
	field.symbols = make([]rune, 1)
	field.symbols[0] = ' '
	field.SetCursorX(0)
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
		window.playerM.endx = window.width
		window.playerM.endy = window.height - 2*window.vMargin - window.artM.endy - 2
	}
	return window.playerM.endx, window.playerM.endy
}

type artModel struct {
	x           int
	y           int
	endx        int
	endy        int
	asciiart    [][]ascii.CharPixel
	style       tcell.Style
	converter   convert.ImageConverter
	placeholder image.Image
	cover       image.Image
}

func (model *artModel) GetBounds() (int, int) {
	return model.endx, model.endy
}

func (model *artModel) MoveCursor(offx, offy int) {
	return
}

func (model *artModel) limitCursor() {
	return
}

func (model *artModel) GetCursor() (int, int, bool, bool) {
	return 0, 0, false, true
}

func (model *artModel) SetCursor(x int, y int) {
	return
}

func (model *artModel) GetCell(x, y int) (rune, tcell.Style, []rune, int) {
	if x > len(model.asciiart[0])-1 || y > len(model.asciiart)-1 {
		return ' ', model.style, nil, 1
	}
	switch window.artDrawingMode {
	case 1:
		return rune(model.asciiart[y][x].Char), tcell.StyleDefault.Background(
			tcell.NewRGBColor(
				int32(model.asciiart[y][x].R),
				int32(model.asciiart[y][x].G),
				int32(model.asciiart[y][x].B))).Foreground(
			tcell.NewRGBColor(
				int32(model.asciiart[y][x].R/2),
				int32(model.asciiart[y][x].G/2),
				int32(model.asciiart[y][x].B/2))), nil, 1
	case 2:
		return rune(model.asciiart[y][x].Char), tcell.StyleDefault.Background(
			tcell.NewRGBColor(
				int32(model.asciiart[y][x].R/2),
				int32(model.asciiart[y][x].G/2),
				int32(model.asciiart[y][x].B/2))).Foreground(
			tcell.NewRGBColor(
				int32(model.asciiart[y][x].R),
				int32(model.asciiart[y][x].G),
				int32(model.asciiart[y][x].B))), nil, 1
	case 3:
		return rune(model.asciiart[y][x].Char), model.style.Background(
			tcell.NewRGBColor(
				int32(model.asciiart[y][x].R),
				int32(model.asciiart[y][x].G),
				int32(model.asciiart[y][x].B))).
			Foreground(window.bgColor), nil, 1
	case 4:
		return rune(model.asciiart[y][x].Char), model.style.Background(
			tcell.NewRGBColor(
				int32(model.asciiart[y][x].R),
				int32(model.asciiart[y][x].G),
				int32(model.asciiart[y][x].B))).
			Foreground(window.fgColor), nil, 1
	case 5:
		return rune(model.asciiart[y][x].Char), model.style.Foreground(
			tcell.NewRGBColor(
				int32(model.asciiart[y][x].R),
				int32(model.asciiart[y][x].G),
				int32(model.asciiart[y][x].B))), nil, 1
	default:
		return ' ', model.style.Background(
			tcell.NewRGBColor(
				int32(model.asciiart[y][x].R),
				int32(model.asciiart[y][x].G),
				int32(model.asciiart[y][x].B))), nil, 1

	}
}

type artArea struct {
	*views.CellView
}

func (art *artArea) Size() (int, int) {
	return window.artM.GetBounds()
}

func (art *artArea) HandleEvent(event tcell.Event) bool {
	switch event := event.(type) {
	case *tcell.EventInterrupt:
		switch i := event.Data().(type) {
		case eventCoverDownloader:
			if i < 0 {
				window.artM.cover = nil
			}
			window.artM.refitArt()
			return true
		}
	case *views.EventWidgetResize:
		if window.hasChangedSize() {
			window.artM.refitArt()
			return true
		}
	}
	// don't pass any events to wrapped widget
	return false
}

func (model *artModel) refitArt() {
	options := convert.Options{
		Ratio:           1.0,
		FixedWidth:      -1,
		FixedHeight:     -1,
		FitScreen:       true,
		StretchedScreen: false,
		Colored:         false,
		Reversed:        false,
	}
	if model.cover == nil {
		model.asciiart = model.converter.Image2CharPixelMatrix(
			model.placeholder, &options)
	} else {
		model.asciiart = model.converter.Image2CharPixelMatrix(
			model.cover, &options)

	}
	model.endx, model.endy = len(model.asciiart[0])-1,
		len(model.asciiart)-1
}

type playerModel struct {
	metadata *album
	dummy    *album
	endx     int
	endy     int
	//currentTrack int
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
		return 0, 0
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
	return window.width, 1
}

type arguments struct {
	tags     []string
	location []string
	sort     string
	flag     int
}

func parseInput(input string) {
	// FIXME `-t ` incorrectly parsed
	commands := strings.Split(input, " ")
	if strings.Contains(commands[0], "http://") || strings.Contains(commands[0], "https://") {
		player.stop()
		player.initPlayer()
		go processMediaPage(commands[0], window.playerM)
		return
	} else if commands[0] == "exit" || commands[0] == "q" || commands[0] == "quit" {
		logFile.WriteString(time.Now().Format(time.ANSIC) + "[ext]:exit with code 0\n")
		app.Quit()
		return
	} else if !strings.HasPrefix(commands[0], "-") {
		window.sendPlayerEvent("search (not implemented)")
		return
	}

	var args arguments

	for i := 0; i < len(commands); i++ {
		if i <= len(commands)-2 && strings.HasPrefix(commands[i], "-") {
			switch commands[i] {
			case "-t", "--tag":
				args.flag = 1
			case "-l", "--location":
				args.flag = 2
			case "-s", "--sort":
				args.flag = 3
			default:
				args.flag = 0
			}
			i++
		}
		switch args.flag {
		case 1:
			args.tags = append(args.tags, commands[i])
		case 2:
			args.location = append(args.location, commands[i])
		case 3:
			if commands[i] == "random" || commands[i] == "date" {
				args.sort = commands[i]
			}
		}
	}
	if len(args.tags) > 0 {
		go processTagPage(args)
	} else {
		window.sendPlayerEvent("no tags to search")
	}
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

	window.artM = &artModel{}
	window.artM.placeholder, err = png.Decode(bytes.NewReader(gopherPNG))
	if err != nil {
		checkFatalError(err)
	}
	window.artM.converter = *convert.NewImageConverter()
	window.artM.refitArt()

	window.playerM = &playerModel{}
	window.playerM.dummy = getDummyData()

	spacer1 := &spacer{views.NewText(), false}
	art := &artArea{views.NewCellView()}
	art.SetModel(window.artM)
	spacer2 := &spacer{views.NewText(), false}
	contentBox := views.NewBoxLayout(views.Vertical)
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
	window.AddWidget(spacer2, 0.0)
	contentBox.AddWidget(spacer3, 0.0)
	contentBox.AddWidget(content, 1.0)
	contentBox.AddWidget(message, 0.0)
	contentBox.AddWidget(field, 0.0)
	window.AddWidget(contentBox, 1.0)
	window.AddWidget(spacer4, 0.0)
	// FIXME: messy
	window.widgets = window.Widgets()
	window.widgets = append(window.widgets, contentBox.Widgets()...)

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
