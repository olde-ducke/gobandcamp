package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/gdamore/tcell/v2/views"
)

type contentArea struct {
	*views.CellView
	currentModel int
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
			if model, ok := content.GetModel().(updatedOnTimer); ok {
				model.updateModel()
			}
		}
		return true

	case *tcell.EventKey:
		switch event.Key() {

		case tcell.KeyCtrlL:
			content.toggleModel(1)
			return true

		case tcell.KeyCtrlP:
			content.toggleModel(2)
			return true

		case tcell.KeyEnter:
			if !window.hideInput {
				return false
			}

			if model, ok := content.GetModel().(selectable); ok {
				player.setTrack(model.getItem())
				return true
			}

		}

	case *tcell.EventInterrupt:
		if event.Data() == nil {
			if model, ok := content.GetModel().(updatedOnTimer); ok {
				model.updateModel()
			}
			app.Update()
			return true
		}

		switch event.Data().(type) {
		case *eventNewItem, *eventNextTrack:
			content.switchModel(content.currentModel)
			return true
		}
	}

	if content.currentModel == 0 || !window.hideInput {
		return false
	}
	return content.CellView.HandleEvent(event)
}

func (content *contentArea) toggleModel(model int) {
	if content.currentModel != model {
		content.switchModel(model)
	} else {
		content.switchModel(0)
	}
}

func (content *contentArea) switchModel(model int) {
	content.currentModel = model

	switch content.currentModel {

	case 0:
		content.SetModel(window.playerM)
		window.playerM.updateModel()

	case 1:
		window.textM.updateText()
		content.SetModel(window.textM)
		// TODO: reset position of view, currently gets stuck wherever
		// it was

	case 2:
		window.playlistM.updateModel()
		content.SetModel(window.playlistM)
		content.SetCursorY(player.currentTrack * 3)
	}
	app.Update()
}

func (content *contentArea) Size() (int, int) {
	return window.getBounds()
}

// TODO: finish or remove
type updatedOnTimer interface {
	updateModel()
}

type selectable interface {
	getItem() int
}

type playerModel struct {
	endx int
	endy int
	text [][]rune
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

// TODO: styling based on some kind of control symbol? might break with some weird unicode combination
// FIXME: truncated dots not in the same style
func (model *playerModel) GetCell(x, y int) (rune, tcell.Style, []rune, int) {
	var ch rune
	if y < len(model.text) {
		if x < len(model.text[y]) {

			// truncate tail of any string that's out of bounds to ...
			if len(model.text[y]) > model.endx && x > model.endx-4 {
				return '.', window.style, nil, 1
			}
			// draw "by" and tags in alt color
			if ((x == 1 || x == 2) && y == 1) || y == 3 {
				return model.text[y][x], window.style.Foreground(window.altColor), nil, 1
			}

			return model.text[y][x], window.style, nil, 1
		}
	}
	return ch, window.style, nil, 1
}

func (model *playerModel) updateModel() {
	var volume string
	var repeats int
	timeStamp := player.getCurrentTrackPosition()
	track := player.currentTrack

	model.endx, model.endy = window.getBounds()

	// FIXME: terrible place for this
	// if for whatever reason we end up with empty playlist
	// go back to initial state
	if window.playlist == nil {
		window.playlist = getDummyData()
		player.totalTracks = 0
		window.artM.cover = nil
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
	generateCharMatrix(text, model.text)

}

func generateCharMatrix(text string, matrix [][]rune) (maxx int, maxy int) {
	x, y := 0, 0
	for _, r := range text {
		if x > maxx {
			maxx = x
		}

		if r == '\r' {
			continue
		} else if r == '\n' {
			x = 0
			y++
			continue
		}

		x++
		matrix[y] = append(matrix[y], r)
	}
	return maxx + 1, y + 1
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
	track := player.currentTrack

	if window.playlist.tracks[track].lyrics == "" {
		model.text = make([][]rune, 1)
		model.text[0] = append(model.text[0], '-', '-', '-', ' ', 'n', 'o', ' ',
			'l', 'y', 'r', 'i', 'c', 's', ' ', 'f', 'o', 'u', 'n', 'd', ' ', '-', '-', '-')
		model.endx = 23
		model.endy = 1
		return
	}

	text := fmt.Sprint("--- ", window.playlist.tracks[track].title, " by ",
		window.playlist.artist, " ---\n", window.playlist.tracks[track].lyrics, "\n--- END ---")
	model.text = make([][]rune, strings.Count(text, "\n")+1)

	model.endx, model.endy = generateCharMatrix(text, model.text)
}

type model struct {
	x        int
	y        int
	onBottom bool
	item     int
	endx     int
	endy     int
	hide     bool
	enab     bool
	loc      string
	text     [][]rune
}

func (m *model) GetBounds() (int, int) {
	return m.endx, m.endy
}

func (model *model) MoveCursor(offx, offy int) {

	if offx != 0 {
		return
	}

	var offset int
	if offy < 0 {
		if model.onBottom {
			offset += (offy - 1) % 3 * 2
			model.onBottom = false
		} else {
			offset += (offy % 3) * 2
		}
	} else if offy > 0 {
		if !model.onBottom {
			offset += (offy + 1) % 3 * 2
			model.onBottom = true
		} else {
			offset += (offy % 3) * 2
		}
	}

	model.y = model.y + offy + offset
	model.item = model.y / 3
	model.limitCursor()
}

func (model *model) limitCursor() {

	if model.y <= 0 {
		model.y = 0
		model.item = 0
		//model.onBottom = false
	}
	if model.y >= model.endy-1 {
		model.y = model.endy - 1
		model.item = window.playlist.totalTracks - 1
		//model.onBottom = true
	}
	window.sendEvent(newDebugMessage(fmt.Sprintf("Cursor is %d,%d, selected:%d, onbottom:%v", model.x, model.y, model.item, model.onBottom)))
}

func (m *model) GetCursor() (int, int, bool, bool) {
	return m.x, m.y, m.enab, !m.hide
}

func (model *model) SetCursor(x int, y int) {

	var offset int
	if y > model.y {
		offset += 2
		model.onBottom = true
	} else if y < model.y {
		model.onBottom = false
	}
	model.y = y + offset
	model.item = y / 3
	model.limitCursor()
}

func (model *model) GetCell(x, y int) (rune, tcell.Style, []rune, int) {
	var ch rune
	var style = window.style
	var returnWholeLine bool
	track := player.currentTrack

	_, cursorY, _, _ := model.GetCursor()
	if model.onBottom {
		cursorY -= 2
	}

	if y >= cursorY && y <= cursorY+2 {
		style = window.style.Reverse(true)
		returnWholeLine = true
	} else if y >= track*3 && y <= track*3+2 {
		style = window.style.Background(window.altColor)
		returnWholeLine = true
	}

	if y < len(model.text) {
		if x < len(model.text[y]) {
			if len(model.text[y]) > model.endx && x > model.endx-4 {
				return '.', style, nil, 1
			}
			return model.text[y][x], style, nil, 1
		}
	}

	if returnWholeLine {
		return ' ', style, nil, 1
	}

	return ch, style, nil, 1
}

func (model *model) updateModel() {
	currentTrack := player.currentTrack

	sbuilder := strings.Builder{}

	//    1 - title
	//
	//    0m0s
	//  ▹ 2 - title
	// ▱▱▱
	// 3s/2m3s
	//    3 - title
	//
	//    0m0s

	var repeats int
	var symbol string
	formatString := "%s %2d - %s\n%s\n%s%s%s\n"
	timeStamp := player.getCurrentTrackPosition()

	duration := window.playlist.tracks[currentTrack].duration
	if duration > 0 {
		repeats = int(timeStamp) * 100 / (int(duration) * 1_000_000_000) * model.endx / 100
	} else {
		repeats = 0
	}

	if window.asciionly {
		symbol = "="
	} else {
		symbol = "\u25b1"
	}

	for n, track := range window.playlist.tracks {
		if n == currentTrack {
			fmt.Fprintf(&sbuilder, formatString,
				player.status,
				n+1,
				track.title,
				strings.Repeat(symbol, repeats), timeStamp, "/",
				(time.Duration(track.duration) * time.Second).Round(time.Second),
			)
		} else {
			fmt.Fprintf(&sbuilder, formatString,
				"  ",
				n+1,
				track.title,
				"", "", "    ",
				(time.Duration(track.duration) * time.Second).Round(time.Second),
			)
		}
	}

	text := sbuilder.String()
	sbuilder.Reset()

	model.text = make([][]rune, strings.Count(text, "\n"))
	generateCharMatrix(text, model.text)
	model.endx, _ = window.getBounds()
	model.endy = len(model.text)
}

func (model *model) getItem() int {
	return model.item
}
