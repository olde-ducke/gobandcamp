package main

import (
	_ "embed"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/gdamore/tcell/v2/views"
)

// TODO: replace with raw string
//go:embed assets/help
var helpText []byte

const (
	welcomeModel int = iota
	playerModel
	lyricsModel
	playlistModel
	helpModel
	resultsModel
)

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

func (model *defaultModel) GetCell(x, y int) (rune, tcell.Style, []rune, int) {
	var ch rune
	style := window.style
	if y < len(model.text) {
		if x < len(model.text[y]) {
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

	// if volume < 5 or player completely muted, display
	// "mute" instead of volume
	var volume string
	if player.muted {
		volume = "mute"
	} else {
		volume = fmt.Sprintf("%3.0f", (100 + player.volume*10))
	}

	// if we are playing track from album and it is not single
	// display from what album track actually comes
	var from, title, album string
	if !window.playlist.single && !window.playlist.album {
		from = " from "
		title = window.playlist.tracks[track].title
		album = window.playlist.title
	} else {
		//from = ""
		title = window.playlist.title
		//album = ""
	}

	fmt.Fprintf(&model.sbuilder, model.formatString,
		title,
		from, album,
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

		// do not count
		if r == '\ue000' || r == '\ue001' {
			x--
		}

		// ignore zero-width spaces and other silly things here
		if r == '\r' || r == '\u200b' {
			continue
		} else if r == '\n' {
			x = 0
			y++
			continue
		}

		if x > maxx {
			maxx = x
		}

		// TODO: add wide non CJK symbols, like '（' for example
		matrix[y] = append(matrix[y], r)
		// FIXME: reallt janky fix for CJK:
		// CJK scripts and symbols:
		// CJK Radicals Supplement (2E80–2EFF)
		// Kangxi Radicals (2F00–2FDF)
		// Ideographic Description Characters (2FF0–2FFF)
		// CJK Symbols and Punctuation (3000–303F)
		// Hiragana (3040–309F)
		// Katakana (30A0–30FF)
		// Bopomofo (3100–312F)
		// Hangul Compatibility Jamo (3130–318F)
		// Kanbun (3190–319F)
		// Bopomofo Extended (31A0–31BF)
		// CJK Strokes (31C0–31EF)
		// Katakana Phonetic Extensions (31F0–31FF)
		// Enclosed CJK Letters and Months (3200–32FF)
		// CJK Compatibility (3300–33FF)
		// CJK Unified Ideographs Extension A (3400–4DBF)
		// Yijing Hexagram Symbols (4DC0–4DFF)
		// CJK Unified Ideographs (4E00–9FFF)

		// and modern(?) Korean:
		// Hangul Syllables (AC00–D7AF)

		// barely tested, places space after symbol, that way tcell doesn't
		// skip every second symbol
		if r >= '\u2E80' && r <= '\u9FFF' || r >= '\uAC00' && r <= '\uD7AF' {
			x++
			matrix[y] = append(matrix[y], ' ')
		}
		x++
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
			return model.text[y][x], window.style, nil, 1
		}
	}
	return ch, tcell.StyleDefault, nil, 1
}

func (model *textModel) create() {
	window.verifyData()
	track := player.currentTrack
	var from string

	if window.playlist.tracks[track].lyrics == "" {
		model.text = make([][]rune, 1)
		model.text[0] = append(model.text[0], '\ue000', 'n', 'o', ' ',
			'l', 'y', 'r', 'i', 'c', 's', ' ', 'f', 'o', 'u', 'n', 'd',
			'\ue001')
		model.endx = 15
		model.endy = 1
		return
	}

	if !window.playlist.single {
		from = " from \ue000" + window.playlist.title + "\ue001"
	}

	text := fmt.Sprint(window.playlist.tracks[track].title, "\n",
		from, " by \ue000", window.playlist.artist, "\ue001\n\n",
		window.playlist.tracks[track].lyrics)
	model.text = make([][]rune, strings.Count(text, "\n")+1)

	model.endx, model.endy = generateCharMatrix(text, model.text)
}

func (model *textModel) update() {
	// TODO: remove?
	window.verifyData()
	return
}

func (model *textModel) getItem() int {
	return -1
}

type welcomeMessage struct {
	*textModel
}

func (model *welcomeMessage) create() {
	text := "\n\nwelcome to \ue000gobandcamp\ue001\nbarebones terminal player for bandcamp\n\npress \ue000[Tab]\ue001 and enter command/url\n    or \ue000[H]\ue001 to display help and controls"
	// NOTE: hardcoded height
	model.text = make([][]rune, 7)
	model.endx, model.endy = generateCharMatrix(text, model.text)
}

type helpMessage struct {
	*textModel
}

func (model *helpMessage) create() {
	return
}

type menuModel struct {
	x            int
	y            int
	onBottom     bool
	item         int
	activeItem   int
	totalItems   int
	endx         int
	endy         int
	hide         bool
	enab         bool
	text         [][]rune
	formatString [3]string
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
		model.item = model.totalItems - 1
	}
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

	_, cursorY, _, _ := model.GetCursor()
	if model.onBottom {
		cursorY -= 2
	}

	if y >= cursorY && y <= cursorY+2 {
		style = window.style.Reverse(true)
		returnWholeLine = true
	} else if y >= model.activeItem*3 && y <= model.activeItem*3+2 {
		style = window.style.Background(window.accentColor)
		returnWholeLine = true
	}

	if y < len(model.text) {
		if x < len(model.text[y]) {
			return model.text[y][x], style, nil, 1
		}
	}

	if returnWholeLine && x < model.endx {
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
	model.activeItem = player.currentTrack
	model.totalItems = window.playlist.totalTracks
	timeStamp := player.getCurrentTrackPosition()
	repeats := progressbarLength(window.playlist.tracks[model.activeItem].duration,
		timeStamp, model.endx)

	for n, track := range window.playlist.tracks {
		// FIXME: this is just bad
		if n == model.activeItem {
			fmt.Fprintf(&model.sbuilder, model.formatString[0],
				window.getPlayerStatus(),
				n+1,
				track.title,
			)
		} else {
			fmt.Fprintf(&model.sbuilder, model.formatString[0],
				"  ",
				n+1,
				track.title,
			)
		}

		if n == model.activeItem {
			fmt.Fprintf(&model.sbuilder, model.formatString[1],
				strings.Repeat(window.getProgressbarSymbol(), repeats),
				"", "", "", "", "", "", "")
		} else {
			var styleStart, styleEnd string
			if n != model.item {
				styleStart = "\ue000"
				styleEnd = "\ue001"
			}

			if window.playlist.single {
				fmt.Fprintf(&model.sbuilder, model.formatString[1],
					"     by ", styleStart, window.playlist.artist, styleEnd,
					"", "", "", "")
			} else {
				fmt.Fprintf(&model.sbuilder, model.formatString[1],
					"     from ", styleStart, window.playlist.title, styleEnd,
					" by ", styleStart, window.playlist.artist, styleEnd,
				)
			}
		}

		if n == model.activeItem {
			fmt.Fprintf(&model.sbuilder, model.formatString[2],
				timeStamp, "/",
				(time.Duration(track.duration) * time.Second).Round(time.Second))
		} else {
			fmt.Fprintf(&model.sbuilder, model.formatString[2],
				"", "    ",
				(time.Duration(track.duration) * time.Second).Round(time.Second))
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

	if model.item > model.totalItems-1 {
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

type searchResultsModel struct {
	*menuModel
}

func (model *searchResultsModel) create() {
	// check if current item is in search results
	// if it is, it will be highlighted in update()
	// player status will display near active item
	var activeFound bool
	url := window.playlist.url
	for i, item := range window.searchResults.Items {
		if url != "" && item.URL == url {
			model.activeItem = i
			activeFound = true
			break
		}
	}
	if !activeFound {
		model.activeItem = -1
	}
	model.update()
}

func (model *searchResultsModel) update() {
	// ▹  active title
	//     by %artist%
	//    %genre%
	//    title
	//     by %artist%
	//    %genre%
	for i, item := range window.searchResults.Items {

		var by, styleStart, styleEnd, playerStatus string
		if item.Type == "a" || item.Type == "t" {
			by = "by"
		}
		if i != model.item && i != model.activeItem {
			styleStart, styleEnd = "\ue000", "\ue001"
		}
		if i == model.activeItem {
			playerStatus = window.getPlayerStatus()
		}

		fmt.Fprintf(&model.sbuilder, "%2s  %s\n     %s %s%s%s\n    %s%s%s\n",
			playerStatus, item.Title,
			by, styleStart, item.Artist, styleEnd,
			styleStart, item.Genre, styleEnd)

		model.totalItems = i + 1
	}

	text := model.sbuilder.String()
	model.sbuilder.Reset()

	model.text = make([][]rune, strings.Count(text, "\n"))
	generateCharMatrix(text, model.text)

	model.endx, _ = window.getBounds()
	model.endy = len(model.text)
}

func (model *searchResultsModel) MoveCursor(offx, offy int) {
	prevPos := model.getItem()
	model.menuModel.MoveCursor(offx, offy)
	currPos := model.getItem()

	// do not spam downloadings/pulling from cache of last first/item in a list
	if currPos != prevPos {
		model.triggerNewDownload(currPos, offy)
	}
}

func (model *searchResultsModel) SetCursor(x, y int) {
	model.menuModel.SetCursor(x, y)
	model.triggerNewDownload(model.getItem(), y)
}

func (model *searchResultsModel) triggerNewDownload(currPos, offy int) {
	if model.item >= model.totalItems-1 && offy > 0 {
		if !window.searchResults.MoreAvailable {
			window.sendEvent(newMessage("nothing else to show"))
		} else {
			// TODO: finish pulling of new result
			window.sendEvent(newMessage("additional results: not implemented"))
		}
	}

	artId := window.searchResults.Items[currPos].ArtId
	if artId == 0 {
		window.sendEvent(newCoverDownloaded(nil, ""))
		window.coverKey = ""
		return
	}

	url := "https://f4.bcbits.com/img/a" +
		strconv.Itoa(artId) + "_7.jpg"
	window.coverKey = url
	go downloadCover(url)
}