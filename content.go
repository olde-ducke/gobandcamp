package main

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/gdamore/tcell/v2/views"
)

type contentArea struct {
	*views.CellView
}

func (content *contentArea) HandleEvent(event tcell.Event) bool {
	switch event := event.(type) {

	case *views.EventWidgetResize:
		if window.hasChangedSize() {
			//app.Update()
			window.checkOrientation()
			window.artM.refitArt()
			// FIXME: not sure if even updates
			window.playerM.updateText()
		}
		return true

	case *tcell.EventInterrupt:

		if event.Data() == nil {
			window.playerM.updateText()
			app.Update()
			return true
		}
	}
	return false
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

	if window.playerM.endy < 0 {
		window.playerM.endy = 0
	}

	if window.playerM.endx < 0 {
		window.playerM.endx = 0
	}

	return window.playerM.endx, window.playerM.endy
}

type playerModel struct {
	metadata *album
	x        int
	y        int
	endx     int
	endy     int
	count    int
	text     [][]rune
}

func (model *playerModel) GetBounds() (int, int) {
	return model.endx, model.endy
}

func (model *playerModel) MoveCursor(offx, offy int) {
	return
}

func (model *playerModel) GetCursor() (int, int, bool, bool) {
	return 0, 0, false, true
}

func (model *playerModel) SetCursor(x int, y int) {
	return
}

func (model *playerModel) GetCell(x, y int) (rune, tcell.Style, []rune, int) {
	var ch rune
	if y < len(model.text) {
		if x < len(model.text[y]) {

			// draw "by" and tags in alt color
			if ((x == 1 || x == 2) && y == 1) || y == 3 {
				return model.text[y][x], window.style.Foreground(window.altColor), nil, 1
			}

			return model.text[y][x], window.style, nil, 1
		}
	}
	return ch, tcell.StyleDefault, nil, 1
}

// returns true and url if any streamable media was found
func (model *playerModel) getURL(track int) (string, bool) {
	if model.metadata.tracks[track].url != "" {
		return model.metadata.tracks[track].url, true
	} else {
		return "", false
	}
}

// cache key = media url without any parameters
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

func (model *playerModel) updateText() {
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
		repeats = int(timeStamp) * 100 / (int(duration) * 1_000_000_000) * model.endx / 100
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

	text := fmt.Sprintf(model.metadata.formatString(track),
		player.status.String(),
		strings.Repeat(symbol, repeats),
		timeStamp,
		volume,
		player.playbackMode.String())

	// NOTE: hardcoded length
	model.text = make([][]rune, 14)
	x, y := 0, 0
	for _, r := range text {
		if r == '\n' {
			y++
			x = 0
			continue
		}

		// FIXME: on start first symbol is '.'
		// other than that, works fine?
		if x > model.endx-1 {
			if len(model.text[y]) > 0 && model.endx == 0 {
				model.text[y][0] = '.'
			} else {
				for i := 1; model.endx-i >= 0 && i < 4; i++ {
					model.text[y][x-i] = '.'
				}
			}
		}
		x++
		model.text[y] = append(model.text[y], r)
	}
}
