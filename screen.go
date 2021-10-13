package main

import (
	"bytes"
	_ "embed"
	"fmt"
	"image"
	"image/png"
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

type windowLayout struct {
	views.BoxLayout
	screen      tcell.Screen
	hideInput   bool
	width       int
	height      int
	placeholder image.Image
	art         *artModel
	converter   convert.ImageConverter
}

func (window *windowLayout) HandleEvent(event tcell.Event) bool {
	switch event := event.(type) {
	case *tcell.EventKey:
		switch event.Key() {
		case tcell.KeyEscape:
			app.Quit()
			return true
		case tcell.KeyTab:
			window.hideInput = !window.hideInput
		}
	}
	return window.BoxLayout.HandleEvent(event)
}

func (window *windowLayout) checkOrientation() {
	if window.width > 2*window.height {
		window.SetOrientation(views.Horizontal)
	} else {
		window.SetOrientation(views.Vertical)
	}
}

// FIXME: on the start ~64 resize events are captured, every actual
// screen resize captures 36, evey key press captures 14
// don't know what's happening here
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
		return false
	}
	field.MakeCursorVisible()
	posX, _, _, _ := field.GetModel().GetCursor()

	switch event := event.(type) {

	case *tcell.EventKey:
		switch event.Key() {

		case tcell.KeyEnter:
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

		case tcell.KeyDelete:
			if posX < len(field.symbols)-1 {
				field.symbols[posX] = 0
				field.symbols = append(field.symbols[:posX],
					field.symbols[posX+1:]...)
				posX++
			}
			field.SetContent(string(field.symbols))

		case tcell.KeyRune:
			field.symbols = append(field.symbols, 0)
			copy(field.symbols[posX+1:], field.symbols[posX:])
			field.symbols[posX] = event.Rune()
			field.SetContent(string(field.symbols))
			field.SetCursorX(posX + 1)
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
	switch event.(type) {
	case *views.EventWidgetResize:
		if window.hasChangedSize() {
			window.checkOrientation()
		}
		return true
	}
	return content.Text.HandleEvent(event)
}

type artModel struct {
	x        int
	y        int
	endx     int
	endy     int
	asciiart [][]ascii.CharPixel
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
		return ' ', tcell.StyleDefault, nil, 1
	}
	return rune(model.asciiart[y][x].Char), tcell.StyleDefault.Background(
		tcell.NewRGBColor(
			int32(model.asciiart[y][x].R),
			int32(model.asciiart[y][x].G),
			int32(model.asciiart[y][x].B))), nil, 1
}

type artArea struct {
	*views.CellView
}

func (art *artArea) Size() (int, int) {
	return window.art.GetBounds()
}

func (art *artArea) HandleEvent(event tcell.Event) bool {
	switch event.(type) {
	case *views.EventWidgetResize:
		if window.hasChangedSize() {
			window.art.refitArt()
			return true
		}
	}
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
	model.asciiart = window.converter.Image2CharPixelMatrix(
		getPlaceholderImage(), &options)
	model.endx, model.endy = len(model.asciiart[0])-1,
		len(model.asciiart)-1
}

func init() {
	var err error
	window.hideInput = true
	window.placeholder, err = png.Decode(bytes.NewReader(gopherPNG))
	if err != nil {
		checkFatalError(err)
	}
	window.converter = *convert.NewImageConverter()
	window.art = &artModel{}
	window.art.refitArt()

	margin := "   "

	spacer1 := views.NewText()
	spacer1.SetText(margin)
	spacer1.SetStyle(tcell.StyleDefault.Foreground(tcell.ColorChocolate).
		Background(tcell.ColorSkyblue))

	art := &artArea{views.NewCellView()}
	art.SetStyle(tcell.StyleDefault.Foreground(tcell.ColorWhiteSmoke).
		Background(tcell.ColorTomato))
	art.SetModel(window.art)

	spacer2 := views.NewText()
	spacer2.SetText(margin)
	spacer2.SetStyle(tcell.StyleDefault.Foreground(tcell.ColorChocolate).
		Background(tcell.ColorSkyblue))

	contentBox := views.NewBoxLayout(views.Vertical)

	spacer3 := views.NewText()
	spacer3.SetText(margin)
	spacer3.SetStyle(tcell.StyleDefault.Foreground(tcell.ColorChocolate).
		Background(tcell.ColorSkyblue))

	content := &contentArea{}
	content.SetStyle(tcell.StyleDefault.Foreground(tcell.ColorDarkSlateGray).
		Background(tcell.ColorLightGoldenrodYellow))

	message := views.NewText()
	message.SetText(" ")
	message.SetStyle(tcell.StyleDefault.Foreground(tcell.ColorDarkSlateGray).
		Background(tcell.ColorPaleGoldenrod))

	field := &textField{}
	field.EnableCursor(!window.hideInput)
	field.HideCursor(window.hideInput)
	field.Clear()
	field.SetStyle(tcell.StyleDefault.Foreground(tcell.ColorDarkSlateGray).
		Background(tcell.ColorYellowGreen))

	spacer4 := views.NewText()
	spacer4.SetText(margin)
	spacer4.SetStyle(tcell.StyleDefault.Foreground(tcell.ColorChocolate).
		Background(tcell.ColorSkyblue))

	window.AddWidget(spacer1, 0.0)
	window.AddWidget(art, 0.0)
	window.AddWidget(spacer2, 0.0)
	contentBox.AddWidget(spacer3, 0.0)
	contentBox.AddWidget(content, 1.0)
	contentBox.AddWidget(message, 0.0)
	contentBox.AddWidget(field, 0.0)
	window.AddWidget(contentBox, 1.0)
	window.AddWidget(spacer4, 0.0)

	// create new screen to gain access to actual terminal dimensions
	window.screen, err = tcell.NewScreen()
	checkFatalError(err)

	app.SetScreen(window.screen)
	app.SetRootWidget(window)

	app.PostFunc(func() {
		go func() {
			for {
				time.Sleep(time.Second / 2)
				window.HandleEvent(tcell.NewEventInterrupt(nil))
			}
		}()
	})
}

func getPlaceholderImage() image.Image {
	return window.placeholder
}
