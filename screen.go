package main

import (
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/gdamore/tcell/v2/views"
)

var app = &views.Application{}
var window = &windowLayout{}

// active widgets that might be recolored
// default widget interface doesn't have .SetStyle()
// spasers named after their position in horizontal layout
const (
	spacerV1 int = iota
	art
	spacerH1
	spacerV2
	content
	message
	field
	spacerV3
)

type recolorable interface {
	views.Widget
	SetStyle(tcell.Style)
}

type windowLayout struct {
	views.BoxLayout
	screen tcell.Screen

	width       int
	height      int
	orientation int
	hMargin     int
	vMargin     int

	hideInput bool

	theme       int
	widgets     [8]recolorable
	bgColor     tcell.Color
	fgColor     tcell.Color
	altColor    tcell.Color
	altColorBuf tcell.Color
	style       tcell.Style
	asciionly   bool

	boundx, boundy int
	playlist       *album
}

// TODO: finish, but not sure if even needed
// if for whatever reason we end up with empty playlist,
// go back to initial state, if called, then something gone
// horribly wrong
func (window *windowLayout) verifyData() (err error) {
	if window.playlist == nil {
		err = errors.New("something went wrong")
		window.sendEvent(newErrorMessage(err))
		window.playlist = getDummyData()
		// player should ignore any sensitive command with 0 tracks
		// window won't load anything, since all links
		// in dummy data are empty
		player.totalTracks = 0
		player.currentTrack = 0
		window.sendEvent(newCoverDownloaded(nil))
		player.stop()
		player.clearStream()
		return err
	}
	return nil
}

func (window *windowLayout) sendEvent(data interface{}) {
	if _, ok := data.(*eventDebugMessage); ok && !*debug {
		return
	}
	window.HandleEvent(tcell.NewEventInterrupt(data))
}

func getNewTrack(track int) {
	if err := window.verifyData(); err != nil {
		return
	}
	if url, streamable := window.playlist.getURL(track); streamable {
		go downloadMedia(url, track)
	} else {
		window.sendEvent(newMessage("track is not available for streaming"))
		player.status = stopped
	}
}

// TODO: finish this function
func (window *windowLayout) Resize() {
	window.width, window.height = window.screen.Size()
	window.checkOrientation()
	window.widgets[art].(*artArea).model.refitArt()
	window.recalculateBounds()
	if model, ok := window.widgets[content].(*contentArea).GetModel().(updatedOnTimer); ok {
		model.updateModel()
	}
	window.BoxLayout.Resize()
}

func (window *windowLayout) HandleEvent(event tcell.Event) bool {
	switch event := event.(type) {

	case *tcell.EventInterrupt:
		switch data := event.Data().(type) {

		// FIXME: for now do not consume these two event types and
		// pass them to child widget, works? but tcell docs say you
		// SHOULD always either return true or false
		case *eventNewItem:
			if data.value() != nil {
				player.stop()
				player.clearStream()
				window.playlist = data.value()
				player.currentTrack = 0
				getNewTrack(player.currentTrack)
				go downloadCover(window.playlist.getImageURL(3))
				player.totalTracks = data.value().totalTracks
			}
			//return true

			// TODO: isn't it possible to call next track on
			// album change? (and get out of range)
		case *eventNextTrack:
			getNewTrack(data.value())

		case *eventTrackDownloaded:
			if err := window.verifyData(); err != nil {
				return false
			}
			track := player.currentTrack
			if data.value() == window.playlist.getTruncatedURL(track) {
				if player.status == playing {
					player.stop()
					player.clearStream()
				}
				player.play(data.value())
				return true
			}
			return false
		}

	case *tcell.EventKey:
		switch event.Key() {

		case tcell.KeyEscape:
			app.Quit()
			return true

		// dumps all parsed metadata from playlist to logfile
		case tcell.KeyCtrlD:
			window.sendEvent(newDebugMessage(fmt.Sprint(window.playlist)))
			return true

		case tcell.KeyCtrlS:
			if *debug {
				window.playlist = nil
			}

		// recolor everything in random colors
		// if debug flag is not set everything in one random style
		case tcell.KeyCtrlT:
			window.altColor = getRandomColor()
			if *debug {
				for _, widget := range window.widgets {
					widget.SetStyle(getRandomStyle())
				}
				window.style = getRandomStyle()
				window.altColor = getRandomColor()
			} else {
				window.setTheme(4)
			}
			return true

		case tcell.KeyRune:
			if window.hideInput {
				switch event.Rune() {

				case 't', 'T':
					window.changeTheme()
					return true

				case 'e', 'E':
					window.asciionly = !window.asciionly
					// RegisterRuneFallback(r rune, subst string)
					// doesn't do anything for me, even in tty
					return true

				default:
					return player.handleEvent(event.Rune())
				}
			}
		}
	}
	return window.BoxLayout.HandleEvent(event)
}

// FIXME: this assumes that font is 1/2 height to width
func (window *windowLayout) checkOrientation() {
	if window.width > 2*window.height {
		window.SetOrientation(views.Horizontal)
		window.orientation = views.Horizontal
	} else {
		window.SetOrientation(views.Vertical)
		window.orientation = views.Vertical
	}
}

func (window *windowLayout) getProgressbarSymbol() string {
	if window.asciionly {
		return "="
	} else {
		return "\u25b1"
	}
}

func (window *windowLayout) getPlayerStatus() string {
	if window.asciionly {
		return [7]string{"[]", " >", "||",
			"<<", ">>", "|<",
			">|"}[player.status]
	} else {
		return player.status.String()
	}
}

func (window *windowLayout) recalculateBounds() {
	artWidth, artHeight := window.widgets[art].Size()
	if window.orientation == views.Horizontal {
		window.boundx = window.width - artWidth - 3*window.hMargin
		window.boundy = window.height - window.vMargin - 2
	} else {
		window.boundx = window.width - 2*window.vMargin
		window.boundy = window.height - 2*window.vMargin - artHeight - 2
	}

	// clamp to zero, otherwise can lead to negative indices
	// in widgets that use these values
	// FIXME: if image not loaded or rather bigger than screen
	// this will not allow anything else to be drawn, to be precise
	// only 1 row will be visible after resizing
	if window.boundx < 0 {
		window.boundx = 0
	}

	if window.boundy < 0 {
		window.boundy = 0
	}
}

func (window *windowLayout) getBounds() (int, int) {
	return window.boundx, window.boundy
}

func getRandomStyle() tcell.Style {
	return tcell.StyleDefault.Foreground(
		getRandomColor()).Background(getRandomColor())
}

func getRandomColor() tcell.Color {
	rand.Seed(time.Now().UnixNano())
	return tcell.NewHexColor(int32(rand.Intn(maxInt32)))
}

func (window *windowLayout) changeTheme() {
	window.theme = (window.theme + 1) % 3
	window.setTheme(window.theme)
}

// TODO: there is app.SetStyle(), but it seems to work as
// expected only on start? look into that one more time
// NOTE: does work for the spacers, but again, only
// first time, after that they get stuck with whatever style was
// set before
func (window *windowLayout) setTheme(theme int) {
	switch theme {

	case 1, 2:
		window.fgColor, window.bgColor = window.bgColor, window.fgColor
		window.style = tcell.StyleDefault.Background(window.bgColor).
			Foreground(window.fgColor)
		window.altColor = window.altColorBuf

	// TODO: theme based on colors from cover
	// case 3:

	// only triggered by pressing Ctrl+T
	case 4:
		window.style = getRandomStyle()
		window.altColor = getRandomColor()

	default:
		window.style = tcell.StyleDefault
		window.altColor = 0
	}

	for _, widget := range window.widgets {
		widget.SetStyle(window.style)
	}
	//checkDrawingMode()
}

type spacer struct {
	*views.Text
	dynamic bool
}

func (s *spacer) Size() (int, int) {
	if s.dynamic && window.orientation != views.Horizontal {
		return window.vMargin, window.vMargin
	}
	return window.hMargin, window.vMargin
}

// TODO: move to art drawer
// if light theme and colored symbols on background color drawing mode
// selected, reverse color drawing option (by default black is basically
// treated as transparent) and redraw image, if any other mode selected
// and reversing is still enabled, reverse to default and redraw,
// looks bad on white either way, but at least is more recognisable
/*func checkDrawingMode() {
	if window.theme == 1 && window.artDrawingMode == 5 {
		if !window.artM.options.Reversed {
			window.artM.options.Reversed = true
			window.artM.refitArt()
		}
	} else if window.artM.options.Reversed {
		window.artM.options.Reversed = false
		window.artM.refitArt()
	}
}*/

func getDummyData() *album {
	return &album{
		title:       "---",
		artist:      "---",
		date:        "---",
		url:         "https://golang.org",
		tags:        "gopher music png",
		totalTracks: 3,
		tracks: []track{{
			trackNumber: 1,
			title:       "---",
			duration:    0.0,
		},
			{
				trackNumber: 2,
				title:       "---",
				duration:    0.0,
			},
			{
				trackNumber: 3,
				title:       "---",
				duration:    0.0,
			}},
	}
}

func init() {
	var err error
	window.hideInput = true
	window.hMargin, window.vMargin = 3, 1
	// by default none of the colors are used, to keep default terminal look
	// only used for light/dark themes
	window.fgColor = tcell.NewHexColor(0xf9fdff)
	window.bgColor = tcell.NewHexColor(0x2b2b2b)
	// actual bandcamp color (one of) is 0x61929c, but windows for some reason
	// gave 0x5f8787 instead of something something grey, which looks
	// rather nice
	window.altColorBuf = tcell.NewHexColor(0x5f8787)
	window.playlist = getDummyData()

	window.widgets[spacerV1] = &spacer{views.NewText(), false}
	window.widgets[spacerH1] = &spacer{views.NewText(), false}
	window.widgets[spacerV2] = &spacer{views.NewText(), true}
	window.widgets[spacerV3] = &spacer{views.NewText(), true}
	contentHLayout := views.NewBoxLayout(views.Horizontal)
	contentVLayoutOuter := views.NewBoxLayout(views.Vertical)
	contentVLayoutInner := views.NewBoxLayout(views.Vertical)

	window.AddWidget(window.widgets[spacerV1], 0.0)
	window.AddWidget(window.widgets[art], 0.0)
	contentHLayout.AddWidget(window.widgets[spacerV2], 0.0)
	contentVLayoutInner.AddWidget(window.widgets[content], 1.0)
	contentVLayoutInner.AddWidget(window.widgets[message], 0.0)
	contentVLayoutInner.AddWidget(window.widgets[field], 0.0)
	contentHLayout.AddWidget(contentVLayoutInner, 0.0)
	contentHLayout.AddWidget(window.widgets[spacerV3], 0.0)
	contentVLayoutOuter.AddWidget(window.widgets[spacerH1], 0.0)
	contentVLayoutOuter.AddWidget(contentHLayout, 0.0)
	window.AddWidget(contentVLayoutOuter, 1.0)

	// create new screen to gain access to actual terminal dimensions
	// works on unix and windows, unlike ascii2image dependency
	window.screen, err = tcell.NewScreen()
	checkFatalError(err)
	app.SetScreen(window.screen)
	app.SetRootWidget(window)
}
