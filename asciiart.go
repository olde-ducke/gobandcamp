package main

import (
	"bytes"
	_ "embed"
	"image"
	"image/color"
	"image/png"
	"sort"

	"github.com/gdamore/tcell/v2"
	"github.com/gdamore/tcell/v2/views"
	"github.com/nfnt/resize"
	"github.com/olde-ducke/image2ascii/ascii"
	"github.com/olde-ducke/image2ascii/convert"
)

const alphaTreshold = 72

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
	if model.asciiart[y][x].A > alphaTreshold {

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
			window.coverBG, window.coverFG, window.coverAccent = art.model.calculatePallet()
			if window.theme == 3 {
				window.setTheme(3)
			} else if window.theme == 4 {
				window.setTheme(4)
			}
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
	_, color, _ := window.style.Decompose()
	if max(color.RGB()) > colorTreshold && model.artDrawingMode == 5 {
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
		// TODO: use sync.Once to decode first time only?
		panic(err)
	}
	return cover
}

// very naive, works out something decent 80-90% of the time
func (model *artModel) calculatePallet() (tcell.Color, tcell.Color, tcell.Color) {
	if model.cover == nil {
		model.cover = getPlaceholderImage()
	}

	img := resize.Resize(25, 25, model.cover, resize.NearestNeighbor)

	colors := make(map[uint8]pixel)
	for y := 0; y < img.Bounds().Dy(); y++ {
		for x := 0; x < img.Bounds().Dx(); x++ {
			r, g, b, a := img.At(x, y).RGBA()

			var px pixel
			if a > alphaTreshold {
				px.r = uint8(r >> 8)
				px.g = uint8(g >> 8)
				px.b = uint8(b >> 8)
				px.v = maxUINT8(px.r, px.g, px.b)
			} else {
				px.r = 0
				px.g = 0
				px.b = 0
				px.v = 0
			}

			// output = append(output, px)
			colors[px.v] = px
		}
	}

	var output []pixel
	for _, value := range colors {
		output = append(output, value)
	}

	v := func(p1, p2 *pixel) bool {
		return p1.v < p2.v
	}

	by(v).Sort(output)
	// window.sendEvent(newDebugMessage(fmt.Sprint(output)))
	return tcell.FromImageColor(color.RGBA{
			R: output[0].r,
			G: output[0].g,
			B: output[0].b,
			A: 255,
		}),
		tcell.FromImageColor(color.RGBA{
			R: output[len(output)/2].r,
			G: output[len(output)/2].g,
			B: output[len(output)/2].b,
			A: 255,
		}),
		tcell.FromImageColor(color.RGBA{
			R: output[len(output)-1].r,
			G: output[len(output)-1].g,
			B: output[len(output)-1].b,
			A: 255,
		})
}

// returns value from HSV for given RGB color
func max(r, g, b int32) int32 {
	maxC := r

	if g > maxC {
		maxC = g
	}

	if b > maxC {
		maxC = b
	}

	return maxC
}

// same, but for uint8
func maxUINT8(r, g, b uint8) uint8 {
	return uint8(max(int32(r), int32(g), int32(b)))
}

type pixel struct {
	r uint8
	g uint8
	b uint8
	v uint8
}

type by func(p1, p2 *pixel) bool

func (by by) Sort(pixels []pixel) {
	ps := &pixelSorter{
		pixels: pixels,
		by:     by,
	}
	sort.Sort(ps)
}

type pixelSorter struct {
	pixels []pixel
	by     func(p1, p2 *pixel) bool
}

func (ps *pixelSorter) Len() int {
	return len(ps.pixels)
}

func (ps *pixelSorter) Swap(i, j int) {
	ps.pixels[i], ps.pixels[j] = ps.pixels[j], ps.pixels[i]
}

func (ps *pixelSorter) Less(i, j int) bool {
	return ps.by(&ps.pixels[i], &ps.pixels[j])
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
	window.coverBG, window.coverFG, window.coverAccent = model.calculatePallet()
	window.widgets[art] = coverArt
}
