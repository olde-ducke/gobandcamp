package main

import (
	"strings"
	"sync"

	"github.com/gdamore/tcell/v2"
	"github.com/gdamore/tcell/v2/views"
)

type contentArea struct {
	currentModel  int
	previousModel int
	models        [6]contentModel
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
		// reset accentColor styling
		altStyle = false
		offset = 0
		for x := 0; x < ex; x++ {
			ch, style, comb, wid := model.GetCell(x+offset, y)

			if en && x == cx && y == cy && sh {
				style = style.Reverse(true)
			}

			//if {}
			// TODO: finish line truncation idea

			// flip flag
			if ch == '\ue000' || ch == '\ue001' {
				altStyle = !altStyle
				offset++
				ch, style, comb, wid = model.GetCell(x+offset, y)
				if ch == '\ue000' || ch == '\ue001' {
					ch = ' '
				}
			}

			if altStyle {
				style = style.Foreground(window.accentColor)
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

			if x+3 > px1 {
				r, _, _, _ := model.GetCell(x+3+offset, y)
				if r != 0 && r != '\ue000' && r != '\ue001' {
					for ; x <= px1; x++ {
						port.SetContent(x, y, '.', nil, style)
					}
					break
				}
			}

			port.SetContent(x, y, ch, comb, style)
			x += wid - 1
		}
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

// GetModel gets the model for this CellView
func (content *contentArea) GetModel() contentModel {
	return content.models[content.currentModel]
}

// SetModel sets the model for this CellView.
func (content *contentArea) SetModel(modelId int) {
	content.models[modelId].create()
	w, h := content.models[modelId].GetBounds()
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
					content.displayMessage()
				}
			default:
				return false
			}

		case tcell.KeyCtrlL:
			content.toggleModel(lyricsModel)
			content.displayMessage()
			return true

		case tcell.KeyCtrlP:
			content.toggleModel(playlistModel)
			content.displayMessage()
			return true

		case tcell.KeyEnter:

			if !window.hideInput {
				return false
			}

			item := content.GetModel().getItem()
			switch content.currentModel {

			case playlistModel:
				if window.playlist != nil {
					if item < len(window.playlist.tracks) {
						player.setTrack(item)
					}
					return true
				}
				return false

			case resultsModel:
				// FIXME: move check to a function?
				// maybe could be deleted, check for valid data
				// is right before sending
				if window.searchResults != nil {
					if item < len(window.searchResults.Items) {
						if url := window.searchResults.Items[item].URL; url != "" {
							if currentURL := window.getItemURL(); url != currentURL {
								go processMediaPage(url)
							} else {
								content.switchModel(playerModel)
							}
						} else { // TODO: remove, it's better to filter by type
							// when parsing
							window.sendEvent(newMessage("not a media item"))
						}
						return true
					}
				}
				return false

			default:
				return false
			}

		// first one works for on unix, second on windows
		// none on both
		case tcell.KeyBackspace2, tcell.KeyBackspace:
			if !window.hideInput {
				return false
			}
			content.switchModel(content.previousModel)
			content.displayMessage()
		}

		if content.currentModel == playerModel || !window.hideInput {
			return false
		}

		if content.handleModelControl(event.Key()) {
			content.GetModel().update()
			return true
		} else {
			return false
		}

	case *eventUpdate:
		if window.playlist == nil {
			model := content.currentModel
			if model == playerModel || model == playlistModel || model == lyricsModel {
				content.switchModel(welcomeModel)
			}
		} else {
			content.GetModel().update()
		}
		return true

	case *eventDisplayMessage:
		content.displayMessage()
		return true

	case *eventNewTagSearch:
		window.searchResults = event.value()
		content.models[resultsModel] = &searchResultsModel{
			&menuModel{
				enab: true,
				hide: true,
			}}
		content.switchModel(resultsModel)
		content.displayMessage()
		return true

	case *eventAdditionalTagSearch:
		if value := event.value(); value != nil {
			window.searchResults.MoreAvailable = value.MoreAvailable
			//window.searchResults.MoreAvailable = value.waiting
			window.searchResults.page += 1
			window.searchResults.Items = append(window.searchResults.Items,
				value.Items...)
			content.switchModel(resultsModel)
			window.sendEvent(newMessage("new items added"))
		}
		window.searchResults.waiting = false
		return true

	case *eventNewItem:
		// switch current model to player or refresh current
		if content.currentModel == welcomeModel ||
			content.currentModel == resultsModel {
			content.toggleModel(content.currentModel)
		} else {
			content.switchModel(content.currentModel)
		}
		return true

	case *eventNewTrack:
		if content.currentModel != playerModel {
			content.switchModel(content.currentModel)
		}
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
	// FIXME: this is a complete mess
	if model != content.currentModel && content.currentModel != welcomeModel {
		content.previousModel = content.currentModel
	}

	// don't set these models on empty playlist
	if window.playlist == nil {
		if model == playerModel || model == playlistModel || model == lyricsModel {
			model = welcomeModel
			window.sendEvent(newMessage("nothing to show"))
		}
	}

	content.currentModel = model
	content.SetModel(model)

	if content.currentModel != resultsModel {
		// TODO: for now, redownload image again
		// no cache for images yet
		if url := window.getImageURL(2); url != "" {
			if window.coverKey != url {
				window.coverKey = url
				go downloadCover(url)
			}
		} else {
			window.sendEvent(newCoverDownloaded(nil, ""))
		}
	}

	switch content.currentModel {

	// FIXME menu on model switch in some cases will only highlight
	// real cursor position, two other rows will be out of view
	// all of these are probably can be fixed by reimplementing
	// viewport from scratch
	case playlistModel:
		content.SetCursorY(player.currentTrack * 3)

	case resultsModel:
		content.previousModel = playerModel
		model := content.GetModel()
		content.SetCursorY(model.getItem() * 3)

	default:
		content.port.MakeVisible(0, 0)
	}
	window.sendEvent(&eventUpdate{})
}

func (content *contentArea) displayMessage() {
	switch content.currentModel {
	case playlistModel:
		window.sendEvent(newMessage("[Backspace] go back [Ctrl+P] return to player"))

	case lyricsModel:
		window.sendEvent(newMessage("[Backspace] go back [Ctrl+L] return to player"))

	case playerModel:
		window.sendEvent(newMessage("[Tab] enable input [H] display help"))

	case helpModel:
		window.sendEvent(newMessage("[Backspace] go back [H] return to player"))

	case resultsModel:
		window.sendEvent(newMessage("[Backspace] return to player"))
	}
}

func (content *contentArea) Size() (int, int) {
	return window.getBounds()
}

func init() {
	player := &defaultModel{
		formatString: "%s\n%s\ue000%s \ue001by \ue000%s\ue001\nreleased %s\n" +
			"\ue000%s\ue001\n\n%2s %2d/%d - %s\n%s" +
			"\n%s/%s\nvolume %4s mode %s\n\n\n\n\n%s",
	}

	lyrics := &textModel{}

	help := &helpMessage{&textModel{}}
	text := string(helpText)
	helpText = make([]byte, 0) // not sure why
	help.text = make([][]rune, strings.Count(text, "\n")+1)
	help.endx, help.endy = generateCharMatrix(text, help.text)

	welcome := &welcomeMessage{&textModel{}}
	text = "\n\nwelcome to \ue000gobandcamp\ue001\nbarebones terminal player for bandcamp\n\npress \ue000[Tab]\ue001 and enter command/url\n    or \ue000[H]\ue001 to display help and controls"
	welcome.text = make([][]rune, 7)
	welcome.endx, welcome.endy = generateCharMatrix(text, welcome.text)

	playlist := &menuModel{enab: true, hide: true,
		formatString: [3]string{
			"%s %2d - %s\n",
			"%s%s%s%s%s%s%s%s\n",
			"%s%s%s\n"},
	}

	results := &searchResultsModel{&menuModel{
		enab:       true,
		hide:       true,
		activeItem: -1,
	}}

	contentWidget := NewCellView()
	contentWidget.models[welcomeModel] = welcome
	contentWidget.models[playerModel] = player
	contentWidget.models[lyricsModel] = lyrics
	contentWidget.models[playlistModel] = playlist
	contentWidget.models[helpModel] = help
	contentWidget.models[resultsModel] = results
	// contentWidget.switchModel(welcomeModel)
	contentWidget.previousModel = playerModel
	window.widgets[content] = contentWidget
}
