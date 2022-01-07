package main

import (
	"fmt"
	"math/rand"
	"strconv"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/gdamore/tcell/v2/views"
)

// TODO: should be parsed from argv/config file (?)
// by default none of the colors are used, to keep default terminal look
// only used for light/dark themes
const (
	accentColor tcell.Color = tcell.ColorIsRGB | tcell.Color(0x61929c) |
		tcell.ColorValid
	bgColor tcell.Color = tcell.ColorIsRGB | tcell.Color(0x2b2b2b) |
		tcell.ColorValid
	fgColor tcell.Color = tcell.ColorIsRGB | tcell.Color(0xf9fdff) |
		tcell.ColorValid
	colorTreshold int32 = 127
)

const maxInt32 = (1 << 31) - 1

var app = &views.Application{}
var window = &windowLayout{}

// FIXME: windows build is now more responsive
// but terminal is still flashing every update
type screen struct {
	tcell.Screen
}

// drop screen updates if queue is full
func (screen *screen) Show() {
	if window.screen.HasPendingEvent() {
		return
	}
	screen.Screen.Show()
}

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

// TODO: remove bgColor and fgColor?
// can be obtained from style
type windowLayout struct {
	views.BoxLayout
	screen tcell.Screen

	width       int
	height      int
	orientation int
	hMargin     int
	vMargin     int

	hideInput     bool
	playerVisible bool

	theme       int
	widgets     [8]recolorable
	bgColor     tcell.Color
	fgColor     tcell.Color
	accentColor tcell.Color
	style       tcell.Style
	asciionly   bool
	imageSize   int

	searchResults *Result
	// TODO: image cache
	coverKey    string
	coverBG     tcell.Color
	coverFG     tcell.Color
	coverAccent tcell.Color

	boundx, boundy int
	playlist       *album
}

func (window *windowLayout) sendEvent(event tcell.Event) {

	if window.screen == nil {
		return
	}

	switch event := event.(type) {

	// FIXME: may cause issues actually
	case *eventDebugMessage:
		if *debug {
			logFile.WriteString(event.When().Format(time.ANSIC) + "[dbg]:" +
				event.String() + "\n")
		} else {
			return
		}

	case *eventUpdate:
		if window.screen != nil {
			// same as with refit art, update models immediately
			// window.widgets[content].HandleEvent(&eventUpdateModel{})
			// same filter as for screen.Show()
			// not sure if needed, but it puts new events in stream
			// so ignore excessive screen updates and events to them
			if window.screen.HasPendingEvent() {
				return
			}
		} else {
			return
		}

	// FIXME: all drawing is dependent of art sizes
	// need to figure out a way to let widgets resize freely,
	// as it is, if art is not refit first, then everything goes
	// to drain
	case *eventRefitArt:
		window.widgets[art].HandleEvent(event)
		return
	}

	// FIXME: probably will fail somehow
	// fails if event queue is full
	// which can lead to some interesting bugs
	// PostEventWait doesn't seem to do anything different
	// previously events were sent directly to root widget,
	// which works fine, but locks everything untill event is processed
	// might be the reason why windows console updates so slow
	err := window.screen.PostEvent(event)
	if err != nil {
		// NOTE/FIXME: deprecated, will be deleted, not safe to use,
		// though tcell.views uses it internally
		// TODO: write to debug file without spamming event stream
		go func() { window.screen.PostEventWait(newErrorMessage(err)) }()
	}
	//window.HandleEvent(event)
}

// TODO: store image format somewhere?, for now only jpg will work,
// image formats stored in separate json unfortunately, never encountered
// anything other than jpeg, but probably there is a reason for bandcamp
// to store image format
// TODO: hardcoded url, might break, template for url should be collected
// somewhere
// ahttps://f4.bcbits.com/img/a<art_id>_nn.jpg
// other images (avatars, etc) stored without type prefix?
// _16, _5 - 700x700, _16 mentioned in browser (?)
// _15     - 135x135
// _14     - 368x368
// _13     - 380x380
// _12     - 138x138
// _11     - 172x172
// _10     - original aspect ratio, scaled down, stored in json as image_src
// _9      - 210x210
// _8      - 124x124
// _7      - 150x150, _7 mentioned in browser (?)
// _6, _3  - 100x100, _3 used as favicon(?)
// _4      - 300x300
// _2      - 350x350
// _1, _0  - original (?)
func (window *windowLayout) getImageURL(artID int) string {
	if artID == 0 {
		return ""
	}

	var s string

	switch window.imageSize {
	case 3:
		s = "_16.jpg"
	case 2:
		s = "_14_.jpg"
	case 1:
		s = "_7.jpg"
	case 0:
		s = "_3.jpg"
	default:
		// return window.playlist.imageSrc
		s = "_10.jpg"
	}

	return "https://f4.bcbits.com/img/a" +
		strconv.Itoa(artID) + s
}

func (window *windowLayout) getItemURL() string {
	if window.playlist == nil {
		return ""
	}
	return window.playlist.url
}

// returns true and url if any streamable media was found
func (window *windowLayout) getTrackURL(track int) (string, bool) {
	if window.playlist == nil {
		return "", false
	}

	if url := window.playlist.tracks[track].url; url != "" {
		return url, true
	} else {
		return "", false
	}
}

func (window *windowLayout) getArtID() int {
	if window.playlist == nil {
		return 0
	}
	return window.playlist.artID
}

func (window *windowLayout) getNewTrack(track int) {
	if url, streamable := window.getTrackURL(track); streamable {
		wg.Add(1)
		go downloadMedia(url, track)
	} else {
		window.sendEvent(newMessage(fmt.Sprintf("track %d is not available for streaming",
			track+1)))
		player.stop()
	}
}

func (window *windowLayout) Resize() {
	window.width, window.height = window.screen.Size()
	window.checkOrientation()
	window.sendEvent(&eventRefitArt{})
	window.sendEvent(&eventUpdate{})
	window.BoxLayout.Resize()
}

func (window *windowLayout) HandleEvent(event tcell.Event) bool {
	switch event := event.(type) {

	case *eventNewItem:
		if event.value() != nil {
			player.stop()
			player.clearStream()
			window.playlist = event.value()
			// FIXME: direct access to player data
			player.currentTrack = 0
			window.getNewTrack(player.currentTrack)

			imageURL := window.getImageURL(window.playlist.artID)
			window.coverKey = imageURL
			wg.Add(1)
			go downloadCover(imageURL)
			player.totalTracks = event.value().totalTracks
			return window.widgets[content].HandleEvent(event)
		}
		return true

		// FIXME: isn't it possible to call next track on
		// album change? (and get out of range)
		// second one fixed?
		// first one won't be fixed for now, not a major problem
	case *eventNewTrack:
		window.getNewTrack(event.value())
		return window.widgets[content].HandleEvent(event)

	case *eventNextTrack:
		player.nextTrack()
		return true

	case *eventTrackDownloaded:
		track := player.currentTrack
		url, ok := window.getTrackURL(track)
		if !ok {
			return false
		}

		if event.value() == getTruncatedURL(url) {
			if player.status == playing {
				player.stop()
				player.clearStream()
			}
			player.play(event.value())
			return true
		}

	case *eventUpdate:
		app.Update()
		return window.widgets[content].HandleEvent(event)

	case *tcell.EventKey:
		switch event.Key() {

		case tcell.KeyEscape:
			app.Quit()
			return true

		// still can't see real difference between
		// screen.Show() and screen.Sync()
		case tcell.KeyF5:
			app.Refresh()
			return true

		// dumps all parsed metadata from playlist to logfile
		case tcell.KeyCtrlD:
			window.sendEvent(newDebugMessage(fmt.Sprint(window.playlist)))
			window.sendEvent(newDebugMessage(fmt.Sprint(window.searchResults)))
			return true

		// forcefully clear all playlist data, even if playback already started
		case tcell.KeyCtrlS:
			if *debug {
				window.playlist = nil
			}

		// recolor everything in random colors
		// if debug flag is not set color everything in one random style
		case tcell.KeyCtrlT:
			window.accentColor = getRandomColor()
			if *debug {
				for _, widget := range window.widgets {
					widget.SetStyle(getRandomStyle())
				}
				window.style = getRandomStyle()
				window.accentColor = getRandomColor()
			} else {
				window.setTheme(5)
			}
			return true

		case tcell.KeyRune:
			if window.hideInput {
				switch event.Rune() {

				case 't', 'T':
					window.changeTheme()
					return true

				case 'e', 'E':
					// RegisterRuneFallback(r rune, subst string)
					// doesn't do anything for me, even in tty
					window.asciionly = !window.asciionly
					return true

				// TODO: remove later
				case 'h', 'H':
					return window.widgets[content].HandleEvent(event)

				// TODO: handle player events here, right now all runes go
				// to player
				default:
					if window.handlePlayerControls(event.Rune()) {
						window.sendEvent(&eventUpdate{})
					}
				}
			}
		}
	}
	return window.BoxLayout.HandleEvent(event)
}

func (window *windowLayout) handlePlayerControls(key rune) bool {
	switch key {
	// TODO: change controls
	case ' ':
		return player.playPause()

	case 'a', 'A':
		return player.seek(false)

	case 'd', 'D':
		return player.seek(true)

	case 's', 'S':
		player.lowerVolume()
		window.displayIfHidden("volume " + player.getVolume())
		return true

	case 'w', 'W':
		player.raiseVolume()
		window.displayIfHidden("volume " + player.getVolume())
		return true

	case 'm', 'M':
		player.mute()
		window.displayIfHidden("volume " + player.getVolume())
		return true

	case 'r', 'R':
		player.nextMode()
		window.displayIfHidden("mode " + player.getPlaybackMode())
		return true

	case 'b', 'B':
		// jump to start instead of previous track if current position
		// after 3 second mark
		if player.getCurrentTrackPosition() > time.Second*3 {
			player.resetPosition()
			return true
		} else {
			return player.skip(false)
		}

	case 'f', 'F':
		return player.skip(true)

	case 'p', 'P':
		if player.isPlaying() {
			player.stop()
			return true
		}
		return false

	default:
		return false
	}
}

func (window *windowLayout) displayIfHidden(message string) {
	if !window.playerVisible {
		window.sendEvent(newMessage(message))
	}
}

// NOTE: this assumes that font is 1/2 height to width
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
			">|"}[player.getStatus()]
	} else {
		return player.getStatus().String()
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
	window.theme = (window.theme + 1) % 5
	window.setTheme(window.theme)
}

// TODO: there is app.SetStyle(), but it seems to work as
// expected only on start? look into that one more time
// NOTE: does work for the spacers, but again, only
// first time, after that they get stuck with whatever style was
// set before
func (window *windowLayout) setTheme(theme int) {
	switch theme {

	case 1:
		window.bgColor, window.fgColor, window.accentColor =
			bgColor, fgColor, accentColor
		window.style = tcell.StyleDefault.Background(window.bgColor).
			Foreground(window.fgColor)

	case 2:
		window.bgColor, window.fgColor, window.accentColor =
			fgColor, bgColor, accentColor
		window.style = tcell.StyleDefault.Background(window.bgColor).
			Foreground(window.fgColor)

	// TODO: theme based on colors from cover
	case 3:
		window.bgColor, window.fgColor, window.accentColor =
			window.coverBG, window.coverFG, window.coverAccent
		window.style = tcell.StyleDefault.Background(window.bgColor).
			Foreground(window.fgColor)

	case 4:
		window.bgColor, window.fgColor, window.accentColor =
			window.coverFG, window.coverBG, window.coverAccent
		window.style = tcell.StyleDefault.Background(window.bgColor).
			Foreground(window.fgColor)

	// only triggered by pressing Ctrl+T
	case 5:
		window.style = getRandomStyle()
		window.accentColor = getRandomColor()

	default:
		window.style = tcell.StyleDefault
		window.accentColor = 0
	}

	for _, widget := range window.widgets {
		widget.SetStyle(window.style)
	}
	window.sendEvent(&eventCheckDrawMode{})
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

/* func getDummyData() *album {
	return &album{
		single:      false,
		album:       true,
		imageSrc:    "",
		title:       "---",
		artist:      "---",
		date:        "---",
		url:         "https://golang.org",
		tags:        "gopher music png",
		totalTracks: 1,
		tracks: []track{{
			trackNumber: 1,
			title:       "---",
			duration:    0.0,
		}},
	}
} */

func init() {
	var err error
	window.hideInput = true
	window.hMargin, window.vMargin = 3, 1
	window.bgColor = bgColor
	window.fgColor = fgColor
	window.imageSize = 1

	window.widgets[spacerV1] = &spacer{views.NewText(), false}
	window.widgets[spacerH1] = &spacer{views.NewText(), false}
	window.widgets[spacerV2] = &spacer{views.NewText(), true}
	window.widgets[spacerV3] = &spacer{views.NewText(), true}
	contentHLayout := views.NewBoxLayout(views.Horizontal)
	contentVLayoutOuter := views.NewBoxLayout(views.Vertical)
	contentVLayoutInner := views.NewBoxLayout(views.Vertical)

	// FIXME: reversed order of widgets breaks for some reason
	// really not sure why, there must be something wrong still
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
	s, err := tcell.NewScreen()
	window.screen = &screen{s}
	checkFatalError(err)
	app.SetScreen(window.screen)
	app.SetRootWidget(window)
}
