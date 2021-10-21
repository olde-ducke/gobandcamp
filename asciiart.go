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

func init() {
	var err error
	window.artM = &artModel{}
	window.artM.placeholder, err = png.Decode(bytes.NewReader(gopherPNG))
	if err != nil {
		checkFatalError(err)
	}
	window.artM.converter = *convert.NewImageConverter()
}