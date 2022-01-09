package main

import (
	_ "embed"
	"fmt"
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
}

func (model *defaultModel) GetCursor() (int, int, bool, bool) {
	return 0, 0, false, true
}

func (model *defaultModel) SetCursor(x int, y int) {
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
	if window.playlist == nil {
		// NOTE: should not get to this point
		return
	}
	track := player.getCurrentTrack()
	timeStamp := player.getCurrentTrackPosition()
	volume := player.getVolume()
	model.endx, model.endy = window.getBounds()
	repeats := progressbarLength(window.playlist.tracks[track].duration,
		timeStamp, model.endx)

	// if we are playing track from album and it is not single
	// display from what album track actually comes
	var from, title, album string
	if !window.playlist.single && !window.playlist.album {
		from = " from "
		title = window.playlist.tracks[track].title
		album = window.playlist.title
	} else {
		title = window.playlist.title
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

		// FIXME: for some reason unicode has separate combining
		// character for diacritics, adding combining characters
		// to the mix breaks everyhing, more reasonable solution
		// is replacing previous character with already existing
		// ones (next in unicode, not completely sure), will work
		// only for hiragana/katakana, will be ignored otherwise
		if r == '\u3099' {
			if n := len(matrix[y]); n > 1 {
				// go back two positions, since hiragana/katakana
				// should be padded with spaces
				n -= 2
				// if symbol is in hiragana or katakana range
				// then replace with next unicode symbol
				if matrix[y][n] >= '\u3040' && matrix[y][n] < '\u30FF' {
					matrix[y][n] += 1
				}
			}
			continue
		}

		// same as above but replace combining character with
		// regular symbol of this diacritic, japanese symbols
		// don't have this one with other symbols
		if r == '\u309A' {
			r = '\u309C'
		}

		// ignore zero-width spaces and other silly things here
		// \uFE00 - \uFE0F - variation selectors, don't print anything
		// selectors for emoticons/emojis
		if r == '\r' || r == '\u200B' || r >= '\uFE00' && r <= '\uFE0F' {
			continue
		} else if r == '\n' {
			x = 0
			y++
			continue
		}

		if x > maxx {
			maxx = x
		}

		matrix[y] = append(matrix[y], r)
		// FIXME: really janky fix for CJK:
		// NOTE: tcell has width argument for SetContent/GetCell etc
		// but it did not fix output (probably doing something wrong)
		// and it is more reasonable to check once per creation, add
		// spaces after wide characters and then forget about the problem?
		// also, truncation checks for empty symbol as signal of line end

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

		// modern(?) Korean:
		// Hangul Syllables (AC00–D7AF)

		// Halfwidth and Fullwidth Forms (FF00–FFEF)

		// barely tested, places space after symbol, that way tcell doesn't
		// skip every second symbol
		// http://www.unicode.org/Public/emoji/1.0//emoji-data.txt
		if r >= '\u2E80' && r <= '\u9FFF' ||
			r >= '\uAC00' && r <= '\uD7AF' ||
			r >= '\uFF00' && r <= '\uFFEF' {
			// r >= '\u2638' && r <= '\u2668' ||
			// r >= '\U0001F600'
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
}

func (model *textModel) GetCursor() (int, int, bool, bool) {
	return 0, 0, false, false
}

func (model *textModel) SetCursor(x int, y int) {
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
	if window.playlist == nil {
		// NOTE: should not get to this point
		return
	}

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
}

func (model *textModel) getItem() int {
	return -1
}

type welcomeMessage struct {
	*textModel
}

func (model *welcomeMessage) create() {
}

type helpMessage struct {
	*textModel
}

func (model *helpMessage) create() {
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
	if window.playlist == nil {
		// NOTE: should not get to this point
		return
	}
	// FIXME: direct acces to player data
	track := player.currentTrack

	model.activeItem = track
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

// TODO?: on certain circumstances there are several
// entries of the same item (exactly the same, and not
// album published by label/etc.)
// FIXME: will crash on empty results but there's no way
// to set this view with empty results?
func (model *searchResultsModel) create() {
	// check if current item is in search results
	// if it is, it will be highlighted in update()
	// player status will display near active item
	var activeFound bool

	url := window.getItemURL()
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

	if len(window.searchResults.Items) == 0 {
		return
	}

	if model.item >= model.totalItems-1 && offy > 0 {
		if !window.searchResults.MoreAvailable {
			window.sendEvent(newMessage("nothing else to show"))
		} else if !window.searchResults.waiting {
			window.searchResults.waiting = true
			wg.Add(1)
			go getAdditionalResults(window.searchResults)
		}
	}

	artID := window.searchResults.Items[currPos].ArtId
	url := window.getImageURL(artID)
	if url == "" {
		window.sendEvent(newCoverDownloaded(nil, ""))
		window.coverKey = ""
		return
	}
	window.coverKey = window.getImageURL(artID)
	wg.Add(1)
	go downloadCover(window.coverKey)
}
