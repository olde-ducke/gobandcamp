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

	// FIXME: only suitable place found for resize events
	// main widget doesn't recognise them for whatever reason
	case *views.EventWidgetResize:
		if window.hasChangedSize() {
			window.checkOrientation()
			window.artM.refitArt()
			window.recalculateBounds()
			window.playerM.updateText()
		}
		return true

	case *tcell.EventKey:
		switch event.Key() {

		case tcell.KeyCtrlL:
			if window.currentModel == 0 {
				window.currentModel = 1
				window.textM.updateText()
				content.SetModel(window.textM)
			} else {
				window.currentModel = 0
				content.SetModel(window.playerM)
			}
			return true
		}

	case *tcell.EventInterrupt:
		if event.Data() == nil {
			window.playerM.updateText()
			app.Update()
			return true
		}

		switch event.Data().(type) {
		case *eventNewItem, *eventNextTrack:
			if window.currentModel == 1 {
				window.textM.updateText()
				content.SetModel(window.textM)
			}
			return true
		}
	}

	if window.currentModel == 0 || !window.hideInput {
		return false
	}
	return content.CellView.HandleEvent(event)
}

func (content *contentArea) Size() (int, int) {
	return window.getBounds()
}

type playerModel struct {
	endx int
	endy int
	text [][]rune
}

func (model *playerModel) GetBounds() (int, int) {
	return window.getBounds()
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

func (model *playerModel) updateText() {
	var volume string
	var repeats int
	timeStamp := player.getCurrentTrackPosition()
	track := player.currentTrack

	model.endx, model.endy = window.getBounds()

	// FIXME: terrible place for this
	if window.playlist == nil {
		window.playlist = getDummyData()
	}

	duration := window.playlist.tracks[track].duration
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

	text := fmt.Sprintf(window.playlist.formatString(track),
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
			x = 0
			y++
			continue
		}

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

type textModel struct {
	endx int
	endy int
	text [][]rune
}

func (model *textModel) GetBounds() (int, int) {
	return model.endx, model.endy
}

func (model *textModel) MoveCursor(offx, offy int) {
	return
}

func (model *textModel) GetCursor() (int, int, bool, bool) {
	return 0, 0, false, false
}

func (model *textModel) SetCursor(x int, y int) {
	return
}

func (model *textModel) GetCell(x, y int) (rune, tcell.Style, []rune, int) {
	var ch rune

	if y < len(model.text) {
		if x < len(model.text[y]) {
			if y == 0 || y == model.endy-1 {
				return model.text[y][x], window.style.Foreground(window.altColor), nil, 1
			}
			return model.text[y][x], window.style, nil, 1
		}
	}
	return ch, tcell.StyleDefault, nil, 1
}

func (model *textModel) updateText() {
	model.text = make([][]rune, 1)
	track := player.currentTrack

	if window.playlist.tracks[track].lyrics == "" {
		model.text[0] = append(model.text[0], '-', '-', '-', ' ', 'n', 'o', ' ',
			'l', 'y', 'r', 'i', 'c', 's', ' ', 'f', 'o', 'u', 'n', 'd', ' ', '-', '-', '-')
		model.endx = 23
		model.endy = 1
		return
	}

	var x, y, maxx = 0, 0, 11

	text := fmt.Sprint("--- ", window.playlist.tracks[track].title, " by ",
		window.playlist.artist, " ---")

	for _, r := range text {
		model.text[y] = append(model.text[y], r)
	}

	y += 2
	model.text = append(model.text, make([]rune, 0), make([]rune, 0))

	for _, r := range window.playlist.tracks[track].lyrics {
		if x > maxx {
			maxx = x
		}

		if r == '\r' {
			continue
		} else if r == '\n' {
			model.text = append(model.text, make([]rune, 0))
			x = 0
			y++
			continue
		}
		model.text[y] = append(model.text[y], r)
		x++
	}

	model.text = append(model.text, make([]rune, 0), make([]rune, 0))
	model.text[len(model.text)-1] = append(model.text[len(model.text)-1], '-', '-', '-', ' ', 'E', 'N', 'D', ' ', '-', '-', '-')

	model.endx, model.endy = maxx+1, len(model.text)
}
