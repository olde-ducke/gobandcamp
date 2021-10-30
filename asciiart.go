package main

import (
	"bytes"
	_ "embed"
	"image"
	"image/png"

	"github.com/gdamore/tcell/v2"
	"github.com/gdamore/tcell/v2/views"
	"github.com/qeesung/image2ascii/ascii"
	"github.com/qeesung/image2ascii/convert"
)

//go:embed assets/gopher.png
var gopherPNG []byte

type artModel struct {
	x         int
	y         int
	endx      int
	endy      int
	asciiart  [][]ascii.CharPixel
	converter convert.ImageConverter
	options   convert.Options
	cover     image.Image
}

func (model *artModel) GetBounds() (int, int) {
	return model.endx, model.endy
}

func (model *artModel) MoveCursor(offx, offy int) {
	return
}

func (model *artModel) GetCursor() (int, int, bool, bool) {
	return 0, 0, false, true
}

func (model *artModel) SetCursor(x int, y int) {
	return
}

func (model *artModel) GetCell(x, y int) (rune, tcell.Style, []rune, int) {
	var ch rune
	if x > len(model.asciiart[0])-1 || y > len(model.asciiart)-1 {
		return ch, window.style, nil, 1
	}

	if window.artDrawingMode != 5 {
		model.options.Reversed = false
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
		return rune(model.asciiart[y][x].Char), window.style.Background(
			tcell.NewRGBColor(
				int32(model.asciiart[y][x].R),
				int32(model.asciiart[y][x].G),
				int32(model.asciiart[y][x].B))).
			Foreground(window.bgColor), nil, 1

	case 4:
		return rune(model.asciiart[y][x].Char), window.style.Background(
			tcell.NewRGBColor(
				int32(model.asciiart[y][x].R),
				int32(model.asciiart[y][x].G),
				int32(model.asciiart[y][x].B))).
			Foreground(window.fgColor), nil, 1

	case 5:
		return rune(model.asciiart[y][x].Char), window.style.Foreground(
			tcell.NewRGBColor(
				int32(model.asciiart[y][x].R),
				int32(model.asciiart[y][x].G),
				int32(model.asciiart[y][x].B))), nil, 1

	default:
		return ' ', window.style.Background(
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

		switch data := event.Data().(type) {
		case *eventCoverDownloaded:
			window.artM.cover = data.value()
			window.artM.refitArt()
			return true
		}
	}
	// don't pass any events to wrapped widget
	return false
}

func (model *artModel) refitArt() {
	if model.cover == nil {
		model.cover = getPlaceholderImage()
	}
	model.asciiart = model.converter.Image2CharPixelMatrix(
		model.cover, &model.options)

	model.endx, model.endy = len(model.asciiart[0])-1,
		len(model.asciiart)-1
}

func getPlaceholderImage() image.Image {
	cover, err := png.Decode(bytes.NewReader(gopherPNG))
	if err != nil {
		checkFatalError(err)
	}
	return cover
}

func init() {
	window.artM = &artModel{}
	window.artM.converter = *convert.NewImageConverter()
	window.artM.options = convert.Options{
		Ratio:           1.0,
		FixedWidth:      -1,
		FixedHeight:     -1,
		FitScreen:       true,
		StretchedScreen: false,
		Colored:         false,
		Reversed:        false,
	}
}
