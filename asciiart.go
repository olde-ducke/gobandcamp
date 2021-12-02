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
}

func (model *artModel) GetCursor() (int, int, bool, bool) {
	return 0, 0, false, true
}

func (model *artModel) SetCursor(x int, y int) {
}

func (model *artModel) GetCell(x, y int) (rune, tcell.Style, []rune, int) {
	var ch rune
	if x > model.endx-1 || y > model.endy-1 {
		return ch, window.style, nil, 1
	}

	// magic number
	if model.asciiart[y][x].A > 72 {

		switch model.artDrawingMode {

		// both image colors, lighter background, darker foreground
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

		// both image colors, darker background, lighter foreground
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

		// image colors on background, app background color on foreground
		case 3:
			return rune(model.asciiart[y][x].Char), window.style.Background(
				tcell.FromImageColor(
					color.RGBA{
						model.asciiart[y][x].R,
						model.asciiart[y][x].G,
						model.asciiart[y][x].B,
						0})).
				Foreground(bgColor), nil, 1

		// image colors on background, app foreground color on foreground
		case 4:
			return rune(model.asciiart[y][x].Char), window.style.Background(
				tcell.FromImageColor(
					color.RGBA{
						model.asciiart[y][x].R,
						model.asciiart[y][x].G,
						model.asciiart[y][x].B,
						0})).
				Foreground(fgColor), nil, 1

		// app background color, image colors on foreground
		case 5:
			return rune(model.asciiart[y][x].Char), window.style.Foreground(
				tcell.FromImageColor(
					color.RGBA{
						model.asciiart[y][x].R,
						model.asciiart[y][x].G,
						model.asciiart[y][x].B,
						0})), nil, 1

		// fill area with spaces and color background to image colors
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
		if window.coverKey == event.key() || event.key() == "" {
			art.model.cover = event.value()
			art.model.refitArt()
			window.recalculateBounds()
			return true
		}

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
	// TODO: finish threshold checking
	_, color, _ := window.style.Decompose()
	if color.Hex() > trColor && model.artDrawingMode == 5 {
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

	// NOTE: this assumes that font is 1/2 height to width
	// TODO: make variable that sets ratio?
	// there seems to be no way to actully get this ratio normal way
	// image2ascii uses same assumption (and ~0.7 for windows?)
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

	model.endx, model.endy = len(model.asciiart[0]),
		len(model.asciiart)
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
