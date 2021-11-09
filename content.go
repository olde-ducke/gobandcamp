package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/gdamore/tcell/v2/views"
)

const (
	playerModel int = iota
	helpModel
	lyricsModel
	playlistModel
	resultsModel
)

type contentArea struct {
	*views.CellView
	currentModel int
	player       *defaultModel
	lyrics       *textModel
	playlist     *menuModel
}

func (content *contentArea) HandleEvent(event tcell.Event) bool {
	switch event := event.(type) {

	case *tcell.EventKey:
		switch event.Key() {

		case tcell.KeyCtrlL:
			content.toggleModel(lyricsModel)
			return true

		case tcell.KeyCtrlP:
			content.toggleModel(playlistModel)
			return true

		case tcell.KeyEnter:
			if !window.hideInput {
				return false
			} else if content.currentModel == playlistModel {
				player.setTrack(content.GetModel().getItem())
				return true
			}
		}

	case *eventUpdate:
		content.GetModel().update()
		app.Update()
		return true

	case *eventNewItem, *eventNextTrack:
		content.switchModel(content.currentModel)
		return true
	}

	if content.currentModel == playerModel || !window.hideInput {
		return false
	}
	return content.CellView.HandleEvent(event)
}

func (content *contentArea) toggleModel(model int) {
	if content.currentModel != model {
		content.switchModel(model)
	} else {
		content.switchModel(playerModel)
	}
}

func (content *contentArea) switchModel(model int) {
	content.currentModel = model
	switch content.currentModel {

	// FIXME: player on some occasions does scroll past end coordinates,
	// FIXME: text position does not reset on track change,
	// FIXME menu on model switch in some cases will only highlight
	// real cursor position, two other rows will be out of view
	// all of these are probably can be fixed by reimplementing
	// viewport from scratch
	case playerModel:
		content.SetModel(content.player)
		content.player.update()

	case lyricsModel:
		content.lyrics.create()
		content.SetModel(content.lyrics)

	case playlistModel:
		content.playlist.update()
		content.SetModel(content.playlist)
		content.SetCursorY(player.currentTrack * 3)
	}
	app.Update()
}

func (content *contentArea) Size() (int, int) {
	return window.getBounds()
}

func (content *contentArea) GetModel() contentModel {
	return content.CellView.GetModel().(contentModel)
}

type contentModel interface {
	views.CellModel
	update()
	create()
	getItem() int
}

type defaultModel struct {
	endx         int
	endy         int
	text         [][]rune
	formatString string
	sbuilder     strings.Builder
}

func (model *defaultModel) GetBounds() (int, int) {
	return model.endx, model.endy
}

func (model *defaultModel) MoveCursor(offx, offy int) {
	return
}

func (model *defaultModel) GetCursor() (int, int, bool, bool) {
	return 0, 0, false, true
}

func (model *defaultModel) SetCursor(x int, y int) {
	return
}

// TODO: styling based on some kind of control symbol? might break with some weird unicode combination
// FIXME: truncated dots not in the same style
func (model *defaultModel) GetCell(x, y int) (rune, tcell.Style, []rune, int) {
	var ch rune
	style := window.style
	if y < len(model.text) {
		if x < len(model.text[y]) {

			// draw "by" and tags in alt color
			if ((x == 1 || x == 2) && y == 1) || y == 3 {
				style = window.style.Foreground(window.altColor)
			}

			// truncate tail of any string that's out of bounds to ...
			if len(model.text[y]) > model.endx && x > model.endx-4 {
				return '.', style, nil, 1
			}

			return model.text[y][x], style, nil, 1
		}
	}
	return ch, window.style, nil, 1
}

func (model *defaultModel) update() {
	window.verifyData()
	track := player.currentTrack
	timeStamp := player.getCurrentTrackPosition()
	model.endx, model.endy = window.getBounds()
	repeats := progressbarLength(window.playlist.tracks[track].duration,
		timeStamp, model.endx)

	var volume string
	if player.muted {
		volume = "mute"
	} else {
		volume = fmt.Sprintf("%3.0f", (100 + player.volume*10))
	}

	fmt.Fprintf(&model.sbuilder, model.formatString,
		window.playlist.title,
		window.playlist.artist,
		window.playlist.date,
		window.playlist.tags,
		window.getPlayerStatus(),
		track+1,
		window.playlist.totalTracks,
		window.playlist.tracks[track].title,
		strings.Repeat(window.getProgressbarSymbol(), repeats),
		timeStamp,
		(time.Duration(window.playlist.tracks[track].duration) * time.Second).Round(time.Second),
		volume, player.playbackMode,
		window.playlist.url,
	)

	text := model.sbuilder.String()
	model.sbuilder.Reset()

	// NOTE: hardcoded length
	model.text = make([][]rune, 14)
	generateCharMatrix(text, model.text)

}

func (model *defaultModel) create() {
	model.update()
}

func (model *defaultModel) getItem() int {
	return -1
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

func (model *textModel) create() {
	window.verifyData()
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

func (model *textModel) update() {
	return
}

func (model *textModel) getItem() int {
	return -1
}

type menuModel struct {
	x            int
	y            int
	onBottom     bool
	item         int
	endx         int
	endy         int
	hide         bool
	enab         bool
	loc          string
	text         [][]rune
	formatString string
	sbuilder     strings.Builder
}

func (m *menuModel) GetBounds() (int, int) {
	return m.endx, m.endy
}

func (model *menuModel) MoveCursor(offx, offy int) {

	if offx != 0 {
		return
	}

	// FIXME: probably it's better to reimplement viewport
	// and its logic from scratch than move fake cursor
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

func (model *menuModel) limitCursor() {

	if model.y <= 0 {
		model.y = 0
		model.item = 0
	}
	if model.y >= model.endy-1 {
		model.y = model.endy - 1
		model.item = window.playlist.totalTracks - 1
	}
	/*window.sendEvent(newDebugMessage(fmt.Sprintf(
	"playlist cursor is %d,%d, onbottom:%v selected item:%d",
	model.x, model.y, model.onBottom, model.item)))*/
}

func (m *menuModel) GetCursor() (int, int, bool, bool) {
	return m.x, m.y, m.enab, !m.hide
}

func (model *menuModel) SetCursor(x int, y int) {

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

func (model *menuModel) GetCell(x, y int) (rune, tcell.Style, []rune, int) {
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

func (model *menuModel) update() {
	//    1 - title
	//
	//    0m0s
	//  ▹ 2 - active title
	// ▱▱▱
	// 3s/2m3s
	//    3 - title
	//
	//    0m0s
	window.verifyData()
	trackn := player.currentTrack
	timeStamp := player.getCurrentTrackPosition()
	repeats := progressbarLength(window.playlist.tracks[trackn].duration,
		timeStamp, model.endx)

	for n, track := range window.playlist.tracks {
		if n == trackn {
			fmt.Fprintf(&model.sbuilder, model.formatString,
				window.getPlayerStatus(),
				n+1,
				track.title,
				strings.Repeat(window.getProgressbarSymbol(), repeats),
				timeStamp, "/",
				(time.Duration(track.duration) * time.Second).Round(time.Second),
			)
		} else {
			fmt.Fprintf(&model.sbuilder, model.formatString,
				"  ",
				n+1,
				track.title,
				"", "", "    ",
				(time.Duration(track.duration) * time.Second).Round(time.Second),
			)
		}
	}

	text := model.sbuilder.String()
	model.sbuilder.Reset()

	model.text = make([][]rune, strings.Count(text, "\n"))
	generateCharMatrix(text, model.text)

	model.endx, _ = window.getBounds()
	model.endy = len(model.text)
}

func (model *menuModel) create() {
	model.update()
}

func (model *menuModel) getItem() int {
	if model.item < 0 {
		model.item = 0
	}

	if model.item > window.playlist.totalTracks-1 {
		model.item = 0
	}

	return model.item
}

func progressbarLength(duration float64, pos time.Duration, width int) int {
	if duration > 0 {
		return int(pos) * 100 / (int(duration) * 1_000_000_000) * width / 100
	} else {
		return 0
	}
}

func init() {
	player := &defaultModel{
		formatString: "%s\n by %s\nreleased %s\n%s\n\n%2s %2d/%d - %s\n%s" +
			"\n%s/%s\nvolume %4s mode %s\n\n\n\n\n%s",
	}

	lyrics := &textModel{}

	playlist := &menuModel{enab: true, hide: true,
		formatString: "%s %2d - %s\n%s\n%s%s%s\n"}

	contentWidget := &contentArea{
		views.NewCellView(), 0, player, lyrics, playlist}
	contentWidget.SetModel(contentWidget.player)
	window.widgets[content] = contentWidget
}
