package main

import (
	_ "embed"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/gdamore/tcell/v2/views"
)

//go:embed assets/help
var helpText []byte

const (
	welcomeModel int = iota
	playerModel
	lyricsModel
	playlistModel
	helpModel
	// resultsModel
)

type contentArea struct {
	currentModel  int
	previousModel int
	models        [5]contentModel
	port          *views.ViewPort
	view          views.View
	style         tcell.Style
	once          sync.Once

	views.WidgetWatchers
}

// Draw draws the content.
func (content *contentArea) Draw() {

	port := content.port
	model := content.GetModel()
	port.Fill(' ', content.style)

	if content.view == nil {
		return
	}
	if model == nil {
		return
	}

	// why???
	/*vw, vh := content.view.Size()
	for y := 0; y < vh; y++ {
		for x := 0; x < vw; x++ {
			content.view.SetContent(x, y, ' ', nil, content.style)
		}
	}*/

	ex, ey := model.GetBounds()
	vx, vy := port.Size()
	_, _, px1, _ := port.GetVisible()
	if ex < vx {
		ex = vx
	}
	if ey < vy {
		ey = vy
	}

	var offset int
	var altStyle bool

	cx, cy, en, sh := model.GetCursor()
	for y := 0; y < ey; y++ {
		for x := 0; x < ex; x++ {
			ch, style, comb, wid := model.GetCell(x+offset, y)

			if en && x == cx && y == cy && sh {
				style = style.Reverse(true)
			}

			// flip flag
			if ch == '\ue000' || ch == '\ue001' {
				altStyle = !altStyle
				offset++
				if x+offset >= ex {
					continue
				}
				ch, style, comb, wid = model.GetCell(x+offset, y)
			}

			if altStyle {
				style = style.Foreground(window.accentColor)
			}

			// truncate tails of strings
			// FIXME: doesn't look good on scrollable texts
			if x+3 > px1 && ch != 0 {
				if r, _, _, _ := model.GetCell(x+3, y); r != 0 { //&& r != ' ' {
					for ; x <= px1; x++ {
						port.SetContent(x, y, '.', nil, style)
					}
					break
				}
			}

			// ignore empty characters completely
			// in start of Draw() we fill screen (buffer) with spaces
			// 2 times and then filling it again outside of actual content
			// one more time? again, why ???
			if ch == 0 {
				//ch = ' '
				//style = content.style
				break
			}

			port.SetContent(x, y, ch, comb, style)
			x += wid - 1
		}
		// reset accentColor styling
		altStyle = false
		offset = 0
	}
}

func (content *contentArea) keyUp() {
	model := content.GetModel()
	if _, _, en, _ := model.GetCursor(); !en {
		content.port.ScrollUp(1)
		return
	}
	model.MoveCursor(0, -1)
	content.MakeCursorVisible()
}

func (content *contentArea) keyDown() {
	model := content.GetModel()
	if _, _, en, _ := model.GetCursor(); !en {
		content.port.ScrollDown(1)
		return
	}
	model.MoveCursor(0, 1)
	content.MakeCursorVisible()
}

func (content *contentArea) keyLeft() {
	model := content.GetModel()
	if _, _, en, _ := model.GetCursor(); !en {
		content.port.ScrollLeft(1)
		return
	}
	model.MoveCursor(-1, 0)
	content.MakeCursorVisible()
}

func (content *contentArea) keyRight() {
	model := content.GetModel()
	if _, _, en, _ := model.GetCursor(); !en {
		content.port.ScrollRight(1)
		return
	}
	model.MoveCursor(+1, 0)
	content.MakeCursorVisible()
}

func (content *contentArea) keyPgUp() {
	model := content.GetModel()
	_, vy := content.port.Size()
	if _, _, en, _ := model.GetCursor(); !en {
		content.port.ScrollUp(vy)
		return
	}
	model.MoveCursor(0, -vy)
	content.MakeCursorVisible()
}

func (content *contentArea) keyPgDn() {
	model := content.GetModel()
	_, vy := content.port.Size()
	if _, _, en, _ := model.GetCursor(); !en {
		content.port.ScrollDown(vy)
		return
	}
	model.MoveCursor(0, +vy)
	content.MakeCursorVisible()
}

func (content *contentArea) keyHome() {
	model := content.GetModel()
	vx, vy := model.GetBounds()
	if _, _, en, _ := model.GetCursor(); !en {
		content.port.ScrollUp(vy)
		content.port.ScrollLeft(vx)
		return
	}
	model.SetCursor(0, 0)
	content.MakeCursorVisible()
}

func (content *contentArea) keyEnd() {
	model := content.GetModel()
	vx, vy := model.GetBounds()
	if _, _, en, _ := model.GetCursor(); !en {
		content.port.ScrollDown(vy)
		content.port.ScrollRight(vx)
		return
	}
	model.SetCursor(vx, vy)
	content.MakeCursorVisible()
}

// MakeCursorVisible ensures that the cursor is visible, panning the ViewPort
// as necessary, if the cursor is enabled.
func (content *contentArea) MakeCursorVisible() {
	model := content.GetModel()
	if model == nil {
		return
	}
	x, y, enabled, _ := model.GetCursor()
	if enabled {
		content.MakeVisible(x, y)
	}
}

// HandleEvent handles events.  In particular, it handles certain key events
// to move the cursor or pan the view.
func (content *contentArea) handleModelControl(key tcell.Key) bool {
	if content.GetModel() == nil {
		return false
	}
	switch key {
	case tcell.KeyUp, tcell.KeyCtrlP:
		content.keyUp()
		return true
	case tcell.KeyDown, tcell.KeyCtrlN:
		content.keyDown()
		return true
	case tcell.KeyRight, tcell.KeyCtrlF:
		content.keyRight()
		return true
	case tcell.KeyLeft, tcell.KeyCtrlB:
		content.keyLeft()
		return true
	case tcell.KeyPgDn:
		content.keyPgDn()
		return true
	case tcell.KeyPgUp:
		content.keyPgUp()
		return true
	case tcell.KeyEnd:
		content.keyEnd()
		return true
	case tcell.KeyHome:
		content.keyHome()
		return true
	}
	return false
}

/*// Size returns the content size, based on the model.
func (content *contentArea) Size() (int, int) {
	// We always return a minimum of two rows, and two columns.
	w, h := content.model.GetBounds()
	// Clip to a 2x2 minimum square; we can scroll within that.
	if w > 2 {
		w = 2
	}
	if h > 2 {
		h = 2
	}
	return w, h
}*/

// GetModel gets the model for this CellView
func (content *contentArea) GetModel() contentModel {
	return content.models[content.currentModel]
}

// SetModel sets the model for this CellView.
func (content *contentArea) SetModel(modelId int) {
	w, h := content.models[modelId].GetBounds()
	//content.model = model
	content.port.SetContentSize(w, h, true)
	content.port.ValidateView()
	content.PostEventWidgetContent(content)
}

// SetView sets the View context.
func (content *contentArea) SetView(view views.View) {
	port := content.port
	port.SetView(view)
	content.view = view
	if view == nil {
		return
	}
	width, height := view.Size()
	content.port.Resize(0, 0, width, height)
	if content.GetModel() != nil {
		w, h := content.models[content.currentModel].GetBounds()
		content.port.SetContentSize(w, h, true)
	}
	content.Resize()
}

// Resize is called when the View is resized.  It will ensure that the
// cursor is visible, if present.
func (content *contentArea) Resize() {
	// We might want to reflow text
	width, height := content.view.Size()
	content.port.Resize(0, 0, width, height)
	content.port.ValidateView()
	content.MakeCursorVisible()
}

// SetCursor sets the the cursor position.
func (content *contentArea) SetCursor(x, y int) {
	content.models[content.currentModel].SetCursor(x, y)
}

// SetCursorX sets the the cursor column.
func (content *contentArea) SetCursorX(x int) {
	_, y, _, _ := content.models[content.currentModel].GetCursor()
	content.SetCursor(x, y)
}

// SetCursorY sets the the cursor row.
func (content *contentArea) SetCursorY(y int) {
	x, _, _, _ := content.models[content.currentModel].GetCursor()
	content.SetCursor(x, y)
}

// MakeVisible makes the given coordinates visible, if they are not already.
// It does this by moving the ViewPort for the CellView.
func (content *contentArea) MakeVisible(x, y int) {
	content.port.MakeVisible(x, y)
}

// SetStyle sets the the default fill style.
func (content *contentArea) SetStyle(s tcell.Style) {
	content.style = s
}

// Init initializes a new CellView for use.
func (content *contentArea) Init() {
	content.once.Do(func() {
		content.port = views.NewViewPort(nil, 0, 0, 0, 0)
		content.style = tcell.StyleDefault
	})
}

// NewCellView creates a CellView.
func NewCellView() *contentArea {
	cv := &contentArea{}
	cv.Init()
	return cv
}

func (content *contentArea) HandleEvent(event tcell.Event) bool {
	switch event := event.(type) {

	case *tcell.EventKey:
		switch event.Key() {

		case tcell.KeyRune:
			switch event.Rune() {
			case 'H', 'h':
				// TODO: remove this check later?
				if window.hideInput {
					content.toggleModel(helpModel)
				}
			default:
				return false
			}

		case tcell.KeyCtrlL:
			content.toggleModel(lyricsModel)
			return true

		case tcell.KeyCtrlP:
			content.toggleModel(playlistModel)
			return true

		case tcell.KeyEnter:

			if !window.hideInput {
				return false
			}
			item := content.GetModel().getItem()
			switch content.currentModel {
			case playlistModel:
				player.setTrack(item)
				return true
			default:
				return false
			}

		case tcell.KeyBackspace2, tcell.KeyBackspace:
			if !window.hideInput {
				return false
			}
			content.switchModel(content.previousModel)
		}

		if content.currentModel == playerModel || !window.hideInput {
			return false
		}
		return content.handleModelControl(event.Key())

	case *eventUpdate:
		content.GetModel().update()
		app.Update()
		return true

	case *eventNewItem, *eventNextTrack:
		if content.currentModel == welcomeModel {
			content.currentModel = playerModel
		}
		content.switchModel(content.currentModel)
		return true
	}
	return false
}

func (content *contentArea) toggleModel(model int) {
	if content.currentModel != model {
		content.switchModel(model)
	} else {
		content.switchModel(playerModel)
	}
}

func (content *contentArea) switchModel(model int) {
	if content.currentModel != welcomeModel {
		content.previousModel = content.currentModel
	}
	content.currentModel = model
	content.GetModel().create()
	content.SetModel(model)
	switch content.currentModel {

	// FIXME menu on model switch in some cases will only highlight
	// real cursor position, two other rows will be out of view
	// all of these are probably can be fixed by reimplementing
	// viewport from scratch
	case playlistModel:
		content.SetCursorY(player.currentTrack * 3)

	default:
		content.port.MakeVisible(0, 0)
	}
	app.Update()
}

func (content *contentArea) Size() (int, int) {
	return window.getBounds()
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

func (model *defaultModel) GetCell(x, y int) (rune, tcell.Style, []rune, int) {
	var ch rune
	style := window.style
	if y < len(model.text) {
		if x < len(model.text[y]) {
			// truncate tail of any string that's out of bounds to ...
			/*if len(model.text[y]) > model.endx && x > model.endx-4 {
				return '.', style, nil, 1
			}*/
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

		// do not count
		if r == '\ue000' || r == '\ue001' {
			x--
		}

		if x > maxx {
			maxx = x
		}

		// ignore zero-width spaces and other silly things here
		if r == '\r' || r == '\u200b' {
			continue
		} else if r == '\n' {
			x = 0
			y++
			continue
		}

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

	if window.playlist.tracks[track].lyrics == "" {
		model.text = make([][]rune, 1)
		model.text[0] = append(model.text[0], '\ue000', '-', '-', '-', ' ', 'n', 'o', ' ',
			'l', 'y', 'r', 'i', 'c', 's', ' ', 'f', 'o', 'u', 'n', 'd', ' ', '-', '-', '-', '\ue001')
		model.endx = 23
		model.endy = 1
		return
	}

	text := fmt.Sprint("\ue000--- ", window.playlist.tracks[track].title, " \ue001by\ue000 ",
		window.playlist.artist, " ---\ue001\n", window.playlist.tracks[track].lyrics, "\n\ue000--- END ---\ue001")
	model.text = make([][]rune, strings.Count(text, "\n")+1)

	model.endx, model.endy = generateCharMatrix(text, model.text)
}

func (model *textModel) update() {
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
		model.item = model.totalItems - 1
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
	//track := player.currentTrack

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
		if n == model.activeItem {
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

func init() {
	player := &defaultModel{
		formatString: "%s\n \ue000by\ue001 %s\nreleased %s\n\ue000%s\ue001\n\n%2s %2d/%d - %s\n%s" +
			"\n%s/%s\nvolume %4s mode %s\n\n\n\n\n%s",
	}

	welcome := &welcomeMessage{&textModel{}}
	lyrics := &textModel{}
	help := &helpMessage{&textModel{}}
	text := string(helpText)
	helpText = make([]byte, 0)
	help.text = make([][]rune, strings.Count(text, "\n")+1)
	help.endx, help.endy = generateCharMatrix(text, help.text)

	playlist := &menuModel{enab: true, hide: true,
		formatString: "%s %2d - %s\n%s\n%s%s%s\n"}

	contentWidget := NewCellView()
	contentWidget.models[welcomeModel] = welcome
	contentWidget.models[playerModel] = player
	contentWidget.models[lyricsModel] = lyrics
	contentWidget.models[playlistModel] = playlist
	contentWidget.models[helpModel] = help
	contentWidget.switchModel(welcomeModel)
	contentWidget.previousModel = playerModel
	window.widgets[content] = contentWidget
}
