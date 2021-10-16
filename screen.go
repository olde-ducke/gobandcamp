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

type eventPlay int
type eventCoverDownloader int
type eventTrackDownloader int

type recolorable interface {
	SetStyle(tcell.Style)
}

type windowLayout struct {
	views.BoxLayout
	screen         tcell.Screen
	hideInput      bool
	width          int
	height         int
	placeholder    image.Image
	artM           *artModel
	playerM        *playerModel
	artDrawingMode int
	orientation    int
	hMargin        int
	vMargin        int
	widgets        []views.Widget
}

func (window *windowLayout) handlePlayerEvent(data interface{}) {
	window.HandleEvent(tcell.NewEventInterrupt(data))
}

func (window *windowLayout) HandleEvent(event tcell.Event) bool {
	switch event := event.(type) {
	case *tcell.EventInterrupt:
		switch data := event.Data().(type) {
		case eventPlay:
			go downloadMedia(window.playerM.media.Trackinfo[player.currentTrack].File.MP3)
		case eventTrackDownloader:
			if data != eventTrackDownloader(player.currentTrack) {
				return false
			}
			player.play(int(data))
		}
	case *tcell.EventKey:
		switch event.Key() {
		case tcell.KeyEscape:
			app.Quit()
			logFile.WriteString(event.When().Format(time.ANSIC) + "[ext]:exit with code 0\n")
			return true
		case tcell.KeyTab:
			window.hideInput = !window.hideInput
		case tcell.KeyRune:
			if window.hideInput && event.Rune() == 'i' {
				window.artDrawingMode = (window.artDrawingMode + 1) % 4
				return true
			}
		case tcell.KeyCtrlD:
			for _, widget := range window.widgets {
				// FIXME: might fail after adding new widgets without
				// this method
				widget.(recolorable).SetStyle(getRandomStyle())
			}
			return true
		}
	}
	return window.BoxLayout.HandleEvent(event)
}

// FIXME ?
func (window *windowLayout) checkOrientation() {
	if window.width > 2*window.height {
		if window.orientation != views.Horizontal {
			window.SetOrientation(views.Horizontal)
			window.orientation = views.Horizontal
		}
	} else if window.orientation != views.Vertical {
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
			app.Update()
			return true
		}
		switch event.Data().(type) {
		case eventPlay:
			content.SetText(
				fmt.Sprint(
					window.playerM.album.Name +
						"\n by " + window.playerM.album.ByArtist["name"].(string) + "\n" +
						window.playerM.album.Tracks.ItemListElement[player.currentTrack].TrackInfo.Name,
				),
			)
			return true
		}
		return false
	}
	return content.Text.HandleEvent(event)
}

type artModel struct {
	x         int
	y         int
	endx      int
	endy      int
	asciiart  [][]ascii.CharPixel
	style     tcell.Style
	converter convert.ImageConverter
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
	switch event.(type) {
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
	model.asciiart = model.converter.Image2CharPixelMatrix(
		getPlaceholderImage(), &options)
	model.endx, model.endy = len(model.asciiart[0])-1,
		len(model.asciiart)-1
}

type playerModel struct {
	album *albumJSON
	media *mediaJSON
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
		}
		return false
	}
	return message.Text.HandleEvent(event)
}

func parseInput(input string) {
	commands := strings.Split(input, " ")
	if strings.Contains(commands[0], "http://") || strings.Contains(commands[0], "https://") {
		player.stop()
		player.initPlayer()
		go processMediaPage(commands[0], true)
		return
	} else if commands[0] == "exit" || commands[0] == "q" || commands[0] == "quit" {
		logFile.WriteString(time.Now().Format(time.ANSIC) + "[ext]:exit with code 0\n")
		app.Quit()
		return
	} else if !strings.HasPrefix(commands[0], "-") {
		window.handlePlayerEvent("search (not implemented)")
		return
	}

	var args struct {
		tags     []string
		location []string
		sort     string
		flag     int
	}

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
	window.handlePlayerEvent(fmt.Sprintf(
		"tag search (not implemented): %s %s %s",
		fmt.Sprint("tags:", args.tags),
		fmt.Sprint("location:", args.location),
		fmt.Sprint("sorting method:", args.sort),
	))
}

func init() {
	var err error
	window.hideInput = true
	window.placeholder, err = png.Decode(bytes.NewReader(gopherPNG))
	if err != nil {
		checkFatalError(err)
	}
	window.artM = &artModel{}
	window.artM.converter = *convert.NewImageConverter()
	window.artM.refitArt()
	window.hMargin, window.vMargin = 3, 1
	window.playerM = &playerModel{}

	spacer1 := &spacer{views.NewText(), false}
	art := &artArea{views.NewCellView()}
	art.SetModel(window.artM)
	spacer2 := &spacer{views.NewText(), false}
	contentBox := views.NewBoxLayout(views.Vertical)
	spacer3 := &spacer{views.NewText(), true}
	content := &contentArea{}
	message := &messageBox{views.NewText()}
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

	// sometimes doesn't work inside main event loop
	//app.PostFunc(func() {
	go func() {
		for {
			time.Sleep(time.Second / 2)
			window.handlePlayerEvent(nil)
		}
	}()
	//})
}

func getPlaceholderImage() image.Image {
	return window.placeholder
}

func getRandomStyle() tcell.Style {
	return tcell.StyleDefault.Foreground(
		tcell.NewHexColor(
			int32(rand.Intn(2_147_483_647)))).Background(
		tcell.NewHexColor(
			int32(rand.Intn(2_147_483_647))))
}
