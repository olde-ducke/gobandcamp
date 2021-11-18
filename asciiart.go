package main

import (
	"bytes"
	_ "embed"
	"image"
	"image/color"
	"image/png"

	"github.com/gdamore/tcell/v2"
	"github.com/gdamore/tcell/v2/views"
	"github.com/olde-ducke/image2ascii/ascii"
	"github.com/olde-ducke/image2ascii/convert"
)

//go:embed assets/gopher.png
var gopherPNG []byte

type artModel struct {
	x              int
	y              int
	endx           int
	endy           int
	asciiart       [][]ascii.CharPixel
	converter      convert.ImageConverter
	options        convert.Options
	cover          image.Image
	artDrawingMode int
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
	if x > model.endx || y > model.endy {
		return ch, window.style, nil, 1
	}

	if model.artDrawingMode != 5 {
		model.options.Reversed = false
	}

	// magic number
	if model.asciiart[y][x].A > 72 {

		switch model.artDrawingMode {

		case 1:
			return rune(model.asciiart[y][x].Char), tcell.StyleDefault.Background(
				tcell.FromImageColor(
					color.RGBA{
						model.asciiart[y][x].R,
						model.asciiart[y][x].G,
						model.asciiart[y][x].B,
						0})).Foreground(
				tcell.FromImageColor(
					color.RGBA{
						model.asciiart[y][x].R / 2,
						model.asciiart[y][x].G / 2,
						model.asciiart[y][x].B / 2,
						0})), nil, 1

		case 2:
			return rune(model.asciiart[y][x].Char), tcell.StyleDefault.Background(
				tcell.FromImageColor(
					color.RGBA{
						model.asciiart[y][x].R / 2,
						model.asciiart[y][x].G / 2,
						model.asciiart[y][x].B / 2,
						0})).Foreground(
				tcell.FromImageColor(
					color.RGBA{
						model.asciiart[y][x].R,
						model.asciiart[y][x].G,
						model.asciiart[y][x].B,
						0})), nil, 1

		case 3:
			return rune(model.asciiart[y][x].Char), window.style.Background(
				tcell.FromImageColor(
					color.RGBA{
						model.asciiart[y][x].R,
						model.asciiart[y][x].G,
						model.asciiart[y][x].B,
						0})).
				Foreground(bgColor), nil, 1

		case 4:
			return rune(model.asciiart[y][x].Char), window.style.Background(
				tcell.FromImageColor(
					color.RGBA{
						model.asciiart[y][x].R,
						model.asciiart[y][x].G,
						model.asciiart[y][x].B,
						0})).
				Foreground(fgColor), nil, 1

		case 5:
			return rune(model.asciiart[y][x].Char), window.style.Foreground(
				tcell.FromImageColor(
					color.RGBA{
						model.asciiart[y][x].R,
						model.asciiart[y][x].G,
						model.asciiart[y][x].B,
						0})), nil, 1

		default:
			return ' ', window.style.Background(
				tcell.FromImageColor(
					color.RGBA{
						model.asciiart[y][x].R,
						model.asciiart[y][x].G,
						model.asciiart[y][x].B,
						0})), nil, 1

		}
	} else {
		return ch, window.style, nil, 1
	}
}

type artArea struct {
	*views.CellView
	model *artModel
}

func (art *artArea) Size() (int, int) {
	return art.model.GetBounds()
}

func (art *artArea) HandleEvent(event tcell.Event) bool {
	switch event := event.(type) {

	case *eventCoverDownloaded:
		art.model.cover = event.value()
		art.model.refitArt()
		window.recalculateBounds()
		return true

	case *eventRefitArt:
		art.model.refitArt()
		window.recalculateBounds()
		return true

	case *eventCheckDrawMode:
		art.model.checkDrawingMode()

	case *tcell.EventKey:
		switch event.Key() {

		case tcell.KeyCtrlA:
			art.model.artDrawingMode = (art.model.artDrawingMode + 1) % 6
			art.model.checkDrawingMode()
			return true
		}

	}
	// don't pass any events to wrapped widget
	return false
}

// if light theme and colored symbols on background color drawing mode
// selected, reverse color drawing option (by default black is basically
// treated as transparent) and redraw image, if any other mode selected
// and reversing is still enabled, reverse to default and redraw,
// looks bad on white either way, but at least is more recognisable
func (model *artModel) checkDrawingMode() {
	if window.theme == 1 && model.artDrawingMode == 5 {
		if !model.options.Reversed {
			model.options.Reversed = true
			model.refitArt()
		}
	} else if model.options.Reversed {
		model.options.Reversed = false
		model.refitArt()
	}
}

func (model *artModel) refitArt() {
	if model.cover == nil {
		model.cover = getPlaceholderImage()
	}

	// FIXME: janky fix for windows
	// ascii2image can't get terminal dimensions on windows and uses
	// zeroes as fixed width/height, which is equivalent to original
	// image size, recalculateBounds() calculates negative dimensions
	// and clamps them to 0, this leads to empty terminal
	// this assumes that font roughly 1/2 (height to width) ratio
	// which is not necessarily like that in all cases?
	// works fine with default fonts on both systems
	if window.orientation == views.Horizontal {
		model.options.FixedHeight = window.height
		model.options.FixedWidth = window.height * 2 * model.cover.Bounds().Dx() /
			model.cover.Bounds().Dy()
	} else {
		model.options.FixedWidth = window.width
		model.options.FixedHeight = window.width / 2 * model.cover.Bounds().Dy() /
			model.cover.Bounds().Dx()
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
	model := &artModel{}
	model.converter = *convert.NewImageConverter()
	model.options = convert.Options{
		Ratio:           1.0,
		FixedWidth:      10,
		FixedHeight:     10,
		FitScreen:       false,
		StretchedScreen: false,
		Colored:         false,
		Reversed:        false,
	}
	coverArt := &artArea{
		views.NewCellView(),
		model,
	}
	coverArt.SetModel(model)
	window.widgets[art] = coverArt
}
