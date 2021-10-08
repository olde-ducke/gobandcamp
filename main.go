package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/faiface/beep"
	"github.com/faiface/beep/effects"
	"github.com/faiface/beep/mp3"
	"github.com/faiface/beep/speaker"
	"github.com/gdamore/tcell"
	"github.com/qeesung/image2ascii/convert"
)

// TODO: struct names and fields are complete mess
type Album struct {
	ByArtist      map[string]interface{} `json:"byArtist"`      // field "name" contains artist/band name
	Name          string                 `json:"name"`          // album title
	DatePublished string                 `json:"datePublished"` // release date
	Image         string                 `json:"image"`         // link to album art
	Tracks        Track                  `json:"track"`         // container for track data
	Tags          []string               `json:"keywords"`      // tags/keywords
	AlbumArt      image.Image
}

type Track struct {
	NumberOfItems   int        `json:"numberOfItems"`   // total number of tracks
	ItemListElement []ItemList `json:"itemListElement"` // further container for track data
}

type ItemList struct {
	Position  int  `json:"position"` // track number
	TrackInfo Item `json:"item"`     // further container for track data
}

type Item struct {
	Name               string     `json:"name"`               // track name
	RecordingOf        Lyric      `json:"recordingOf"`        // container for lyrics
	Duration           string     `json:"duration"`           // string representation of duration P##H##M##S:
	AdditionalProperty []Property `json:"additionalProperty"` // list of containers for additional info (link to media, duration in float, and useless info)
}

type Lyric struct {
	Lyrics map[string]string `json:"lyrics"` // field "text" contains actual lyrics
}

type Property struct {
	Name  string      `json:"name"`
	Value interface{} `json:"value"`
}

type playbackMode int

const (
	normal playbackMode = iota
	repeat
	repeatOne
	random
)

func (mode playbackMode) String() string {
	return [4]string{"Normal", "Repeat", "Repeat One", "Random"}[mode]
}

type playbackStatus int

const (
	stopped playbackStatus = iota
	playing
	paused
	seekBWD
	seekFWD
	skipBWD
	skipFWD
)

// □ ▹ ▯▯ ◃◃ ▹▹ ▯◃ ▹▯
func (status playbackStatus) String() string {
	return [7]string{" \u25a1", " \u25b9", "\u25af\u25af",
		"\u25c3\u25c3", "\u25b9\u25b9", "\u25af\u25c3",
		"\u25b9\u25af"}[status]
}

type mediaStream struct {
	sampleRate beep.SampleRate
	streamer   beep.StreamSeekCloser
	ctrl       *beep.Ctrl
	resampler  *beep.Resampler
	volume     *effects.Volume
}

var cachedResponses map[int][]byte

type playback struct {
	currentTrack int
	albumList    *Album // not a list at the moment
	stream       *mediaStream
	format       beep.Format

	status        playbackStatus
	playbackMode  playbackMode
	currentPos    time.Duration
	latestMessage string
	volume        float64
	muted         bool

	event chan interface{}
}

type screenRect struct {
	minX, minY, maxX, maxY int
}

type screenDrawer struct {
	textArea  screenRect
	screen    tcell.Screen
	style     tcell.Style
	margin    int // left margin
	artMode   int
	lightMode bool
	bgColor   tcell.Color
	fgColor   tcell.Color
}

func (player *playback) changePosition(pos int) {
	newPos := player.stream.streamer.Position()
	newPos += pos
	if newPos < 0 {
		newPos = 0
	}
	if newPos >= player.stream.streamer.Len() {
		newPos = player.stream.streamer.Len() - 1
	}
	if err := player.stream.streamer.Seek(newPos); err != nil {
		// sometime reports errors, for example this one:
		// https://github.com/faiface/beep/issues/116
		player.latestMessage = err.Error()
	}
}

func newStream(sampleRate beep.SampleRate, streamer beep.StreamSeekCloser,
	playerVolume float64, muted bool) *mediaStream {
	ctrl := &beep.Ctrl{Streamer: streamer}
	resampler := beep.Resample(4, 44100, sampleRate, ctrl)
	volume := &effects.Volume{Streamer: resampler, Base: 2, Volume: playerVolume, Silent: muted}
	return &mediaStream{sampleRate, streamer, ctrl, resampler, volume}
}

func (player *playback) getNewTrack(trackNumber int) {
	if len(player.albumList.Tracks.ItemListElement) == 0 {
		player.latestMessage = "No album data was found"
		return
	}
	item := player.albumList.Tracks.ItemListElement[trackNumber]
	filename := fmt.Sprint(player.albumList.ByArtist["name"], " - ", item.TrackInfo.Name, ".mp3")

	if cachedResponses[trackNumber] != nil {
		player.latestMessage = fmt.Sprint(filename, " - Cached")
		player.event <- trackNumber
		return
	}

	player.latestMessage = fmt.Sprint(filename, " - Fetching...")
	for _, value := range item.TrackInfo.AdditionalProperty {
		// not all tracks are available for streaming,
		// there is a `streaming` field in JSON
		// but tracks that haven't been published yet
		// (preorder items with some tracks available
		// for streaming) don't have it at all
		if value.Name == "file_mp3-128" {
			response, err := http.DefaultClient.Do(createNewRequest(value.Value.(string)))
			if err != nil {
				return
			}
			if response.StatusCode > 299 {
				player.latestMessage = fmt.Sprintf(
					"Request failed with status code: %d\n", response.StatusCode)
				return
			}
			player.latestMessage = fmt.Sprint(filename, " - ", response.Status, " Downloading...")

			cachedResponses[trackNumber], err = io.ReadAll(response.Body)
			response.Body.Close()
			if err != nil {
				player.latestMessage = err.Error()
				return
			}
			player.latestMessage = fmt.Sprint(filename, " - Done")
			player.event <- trackNumber
			return
		}
	}
	player.latestMessage = "Track is currently not available for streaming"
}

func (player *playback) getCurrentTrackPosition() time.Duration {
	return player.format.SampleRate.D(player.stream.
		streamer.Position()).Round(time.Second)
}

func (player *playback) skip(forward bool) {
	if player.playbackMode == random {
		player.nextTrack()
		return
	}
	player.stop()
	if forward {
		player.currentTrack = (player.currentTrack + 1) %
			player.albumList.Tracks.NumberOfItems
		player.status = skipFWD
	} else {
		player.currentTrack = (player.albumList.Tracks.NumberOfItems +
			player.currentTrack - 1) %
			player.albumList.Tracks.NumberOfItems
		player.status = skipBWD
	}
	go player.getNewTrack(player.currentTrack)
}

func (player *playback) nextTrack() {
	player.stop()
	switch player.playbackMode {
	case random:
		rand.Seed(time.Now().UnixNano())
		player.currentTrack = rand.Intn(player.albumList.Tracks.NumberOfItems)
		go player.getNewTrack(player.currentTrack)
	case repeatOne:
		go player.getNewTrack(player.currentTrack)
	case repeat:
		player.skip(true)
	case normal:
		if player.currentTrack == player.albumList.Tracks.NumberOfItems-1 {
			return
		}
		player.skip(true)
	}
	player.status = skipFWD
}

func (player *playback) stop() {
	speaker.Clear()
	if player.stream != nil {
		err := player.stream.streamer.Close()
		if err != nil {
			player.latestMessage = err.Error()
		}
	}
	player.status = stopped
}

// response.Body doesn't implement Seek() method
// beep isn't bothered by this, but trying to
// call Seek() will fail since Len() will always return 0
// by using bytes.Reader and implementing empty Close() method
// we get io.ReadSeekCloser, which satisfies requirements of beep streamers
// (needs ReadCloser) and implements Seek() method
type bytesRSC struct {
	*bytes.Reader
}

func (c bytesRSC) Close() error {
	return nil
}

func (drawer *screenDrawer) reDrawMetaData(player *playback) {

	drawer.screen.Fill(' ', drawer.style)

	drawer.textArea.minX = drawer.redrawArt(player) + drawer.margin*2
	drawer.textArea.maxX, drawer.textArea.maxY = drawer.screen.Size()
	drawer.updateTextData(player)
	drawer.screen.Sync()
}

// draw album art
func (drawer *screenDrawer) redrawArt(player *playback) int {
	art := convert.NewImageConverter().Image2CharPixelMatrix(
		player.albumList.AlbumArt, &convert.DefaultOptions)

	x, y := drawer.margin, 0
	for _, pixelY := range art {
		for _, pixelX := range pixelY {
			switch drawer.artMode {
			// fill all cells with colourful spaces
			case 0:
				drawer.screen.SetContent(x, y, ' ', nil,
					drawer.style.Background(tcell.NewRGBColor(int32(pixelX.R),
						int32(pixelX.G), int32(pixelX.B))))
			// foreground is darker shade, background lighter shade
			case 1:
				style := tcell.StyleDefault.Background(
					tcell.NewRGBColor(int32(pixelX.R),
						int32(pixelX.G), int32(pixelX.B))).Foreground(
					tcell.NewRGBColor(int32(pixelX.R)/2,
						int32(pixelX.G)/2, int32(pixelX.B)/2))
				drawer.screen.SetContent(x, y, rune(pixelX.Char), nil, style)
			// opposite
			case 2:
				style := tcell.StyleDefault.Background(
					tcell.NewRGBColor(int32(pixelX.R)/2,
						int32(pixelX.G)/2, int32(pixelX.B)/2)).Foreground(
					tcell.NewRGBColor(int32(pixelX.R),
						int32(pixelX.G), int32(pixelX.B)))
				drawer.screen.SetContent(x, y, rune(pixelX.Char), nil, style)
			// draws art with colourful symbols on background color
			case 3:
				drawer.screen.SetContent(x, y, rune(pixelX.Char), nil,
					drawer.style.Foreground(tcell.NewRGBColor(int32(pixelX.R),
						int32(pixelX.G), int32(pixelX.B))))
			// draw colourful background with dark theme symbols
			case 4:
				style := tcell.StyleDefault.Foreground(drawer.bgColor)
				drawer.screen.SetContent(x, y, rune(pixelX.Char), nil,
					style.Background(tcell.NewRGBColor(int32(pixelX.R),
						int32(pixelX.G), int32(pixelX.B))))
			// same, but with light theme symbols
			case 5:
				style := tcell.StyleDefault.Foreground(drawer.fgColor)
				drawer.screen.SetContent(x, y, rune(pixelX.Char), nil,
					style.Background(tcell.NewRGBColor(int32(pixelX.R),
						int32(pixelX.G), int32(pixelX.B))))
			}
			x++
		}
		x = drawer.margin
		y++
	}
	return len(art[0])
}

func (drawer *screenDrawer) updateTextData(player *playback) {
	// TODO: replace this mess with tcell/views?

	messagePos := drawer.textArea.maxY - 2
	for i := 1; i <= messagePos; i++ {
		drawer.clearString(drawer.textArea.minX, i)
	}

	if messagePos > 10 {
		drawer.drawString(drawer.textArea.minX, drawer.textArea.maxY-2,
			player.latestMessage)
	}

	var sbuilder strings.Builder
	if len(player.albumList.Tracks.ItemListElement) == 0 {
		drawer.screen.Show()
		return
	}
	item := player.albumList.Tracks.ItemListElement[player.currentTrack]

	fmt.Fprintf(&sbuilder, "Artist: %s", player.albumList.ByArtist["name"])
	drawer.drawString(drawer.textArea.minX, 1, sbuilder.String())
	sbuilder.Reset()

	fmt.Fprintf(&sbuilder, "Album: %s", player.albumList.Name)
	drawer.drawString(drawer.textArea.minX, 2, sbuilder.String())
	sbuilder.Reset()

	fmt.Fprintf(&sbuilder, "Release Date: %s", player.albumList.DatePublished[:11])
	drawer.drawString(drawer.textArea.minX, 3, sbuilder.String())
	sbuilder.Reset()

	fmt.Fprint(&sbuilder, player.albumList.Tags)
	drawer.drawString(drawer.textArea.minX, 4, sbuilder.String())
	sbuilder.Reset()

	fmt.Fprintf(&sbuilder, "%2s %2d/%d - %s", player.status.String(),
		item.Position, player.albumList.Tracks.NumberOfItems,
		item.TrackInfo.Name)
	drawer.drawString(drawer.textArea.minX, 6, sbuilder.String())
	sbuilder.Reset()

	var seconds float64
	for _, value := range item.TrackInfo.AdditionalProperty {
		if value.Name == "duration_secs" {
			seconds = value.Value.(float64)
		}
	}

	if seconds != 0 {
		var repeats int = int(100*player.currentPos/
			time.Duration(seconds*1_000_000_000)) *
			(drawer.textArea.maxX - drawer.textArea.minX - drawer.margin) /
			100

		drawer.drawString(drawer.textArea.minX, 7, strings.Repeat("\u25b1", repeats))
		sbuilder.Reset()
	}

	fmt.Fprintf(&sbuilder, "%s/%s", player.currentPos,
		time.Duration(seconds*1_000_000_000).Round(time.Second))
	drawer.drawString(drawer.textArea.minX, 8, sbuilder.String())
	sbuilder.Reset()

	if player.muted {
		fmt.Fprintf(&sbuilder, "Volume: Mute")
	} else {
		fmt.Fprintf(&sbuilder, "Volume: %3.0f%%", (100 + player.volume*10))
	}
	fmt.Fprintf(&sbuilder, " Mode: %s", player.playbackMode.String())
	drawer.drawString(drawer.textArea.minX, 9, sbuilder.String())
	sbuilder.Reset()

	drawer.screen.Show()
}

func (drawer *screenDrawer) drawString(x, y int, str string) {
	for _, r := range str {
		if x == drawer.textArea.maxX-drawer.margin {
			x -= 3
			for i := x; i < drawer.textArea.maxX-drawer.margin; i++ {
				drawer.screen.SetContent(i, y, '.', nil, drawer.style)
			}
			return
		}
		drawer.screen.SetContent(x, y, r, nil, drawer.style)
		x++
	}
}

func (drawer *screenDrawer) clearString(x, y int) {
	for i := x; i < drawer.textArea.maxX-drawer.margin; i++ {
		drawer.screen.SetContent(i, y, ' ', nil, drawer.style)
	}
}

func reportError(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func createNewRequest(link string) *http.Request {
	request, err := http.NewRequest("GET", link, nil)
	if err != nil {
		// If we can't even create new request, then something
		//gone horribly wrong
		reportError(err)
	}
	// pretend that we are Chrome on Win10
	request.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/93.0.4577.82 Safari/537.36")
	// set mobile view, html weights a bit less than desktop version
	request.Header.Set("Cookie", "mvp=p")
	return request
}

func (player *playback) downloadCover() {
	// images requests over https fail with EOF error
	// for me lately, not even official app can download
	// covers/avatars/etc, request doesn't fail over http
	response, err := http.DefaultClient.Do(createNewRequest(player.albumList.Image))
	if err != nil {
		player.latestMessage = fmt.Sprint("https://... image request failed with error:",
			err, "trying http://...")
	}
	httpLink := strings.Replace(player.albumList.Image, "https://", "http://", 1)
	response, err = http.DefaultClient.Do(createNewRequest(httpLink))
	if err != nil {
		reportError(err)
	}
	defer response.Body.Close()

	switch response.Header.Get("Content-Type") {
	case "image/jpeg":
		player.albumList.AlbumArt, err = jpeg.Decode(response.Body)
		if err != nil {
			player.albumList.AlbumArt = image.NewNRGBA(image.Rect(0, 0, 1, 1))
			player.event <- err.Error()
			return
		}
	/*
		case "image/png":
			player.albumList.AlbumArt, _ = png.Decode(response.Body)
	*/
	default:
		player.albumList.AlbumArt = image.NewNRGBA(image.Rect(0, 0, 1, 1))
		player.event <- "cover is not jpeg"
	}
	player.event <- "Album cover downloaded"
}

func parseJSON(jsonstring string) *Album {
	var album Album
	// TODO: track and album pages are different, for now only album pages supported
	err := json.Unmarshal([]byte(jsonstring), &album)
	if err != nil {
		reportError(err)
	}
	return &album
}

func (drawer *screenDrawer) initScreen(player *playback) {
	var err error
	drawer.screen, err = tcell.NewScreen()
	reportError(err)
	err = drawer.screen.Init()
	reportError(err)
	if drawer.lightMode {
		drawer.style = tcell.StyleDefault.
			Foreground(drawer.bgColor).
			Background(drawer.fgColor)
	} else {
		drawer.style = tcell.StyleDefault.
			Foreground(drawer.fgColor).
			Background(drawer.bgColor)
	}
	drawer.screen.Fill(' ', drawer.style)
	drawer.reDrawMetaData(player)
	drawer.margin = 3
}

func (player *playback) initPlayer(album *Album) {
	cachedResponses = make(map[int][]byte)
	// keeps volume and playback mode from previous playback
	*player = playback{playbackMode: player.playbackMode, volume: player.volume,
		muted: player.muted, albumList: album}
	player.albumList.AlbumArt = image.NewRGBA(image.Rect(0, 0, 1, 1))
	player.event = make(chan interface{})
	go player.downloadCover()
	go player.getNewTrack(player.currentTrack)
}

func getAlbumPage(link string) (jsonString string, err error) {
	response, err := http.DefaultClient.Do(createNewRequest(link))
	if err != nil {
		return "", err
	}
	if response.StatusCode > 299 {
		return "", errors.New(
			fmt.Sprintf("Request failed with status code: %d\n",
				response.StatusCode),
		)
	}
	body, err := io.ReadAll(response.Body)
	// seems reasonable to crash, if we can't close reader
	reportError(response.Body.Close())
	if err != nil {
		fmt.Println("Error:", err.Error())
		return "", err
	}

	// not all artists are hosted on bandname.bandcamp.com,
	// deal with aliases by reading canonical names from response
	if !strings.Contains(response.Header.Get("Link"),
		"bandcamp.com") {
		return "", errors.New("Response came not from bandcamp.com")
	}

	reader := bytes.NewBuffer(body)

	for {
		jsonString, err = reader.ReadString('\n')
		if err != nil {
			fmt.Println(err.Error())
			return "", err
		}
		if strings.Contains(jsonString, "application/ld+json") {
			jsonString, err = reader.ReadString('\n')
			if err != nil {
				return "", err
			}
			break
		}
	}
	return jsonString, nil
}

func handleInput(message string) (jsonString string) {
	for {
		stdinReader := bufio.NewReader(os.Stdin)
		fmt.Println(message)
		input, err := stdinReader.ReadString('\n')
		reportError(err)
		switch input {
		case "\n":
			return ""
		case "exit\n", "q\n":
			return "q"
		default:
			jsonString, err = getAlbumPage(strings.Trim(input, "\n"))
		}
		if err == nil {
			break
		} else {
			fmt.Println("Error:", err)
		}
	}
	return jsonString
}

func init() {
	sr := beep.SampleRate(44100)
	speaker.Init(sr, sr.N(time.Second/10))
}

func main() {
	var player playback
	// FIXME: behaves weird after coming from suspend (high CPU load)
	// FIXME: device does not reinitialize after suspend
	// FIXME: takes device to itself, doesn't allow any other program to use it, and can't use it, if device is already being used
	var jsonString string
	for jsonString == "" {
		jsonString = handleInput("Enter bandcamp album link, type `exit` or q to quit")
		if jsonString == "q" {
			os.Exit(0)
		}
	}

	player.initPlayer(parseJSON(jsonString))

	drawer := screenDrawer{
		fgColor: tcell.NewHexColor(0xf9fdff),
		bgColor: tcell.NewHexColor(0x2b2b2b),
	}
	drawer.initScreen(&player)
	ticker := time.NewTicker(time.Second / 2)
	timer := ticker.C
	tcellEvent := make(chan tcell.Event)
	// FIXME: probably breaks something: store real player status
	// and display new status while key is held down, then update it
	// on next timer tick (tcell can't tell when key is released)
	var buf playbackStatus

	// doesn't fail without screen and continues to work
	// after reinitialization normally
	// does it even end?
	go func() {
		for {
			tcellEvent <- drawer.screen.PollEvent()
		}
	}()

	// event loop
	for {
		select {
		case <-timer:
			if player.status == seekBWD || player.status == seekFWD {
				player.status = buf
			}
			if player.format.SampleRate != 0 {
				speaker.Lock()
				player.currentPos = player.getCurrentTrackPosition()
				speaker.Unlock()
				drawer.updateTextData(&player)
			}

		// handle player events
		case value := <-player.event:
			switch value.(type) {
			case bool:
				player.nextTrack()
				drawer.updateTextData(&player)
			case int:
				if value.(int) == player.currentTrack && player.status != playing {
					streamer, format, err := mp3.Decode(&bytesRSC{bytes.
						NewReader(cachedResponses[value.(int)])})
					if err != nil {
						player.latestMessage = fmt.Sprint(err)
						continue
					}
					player.format = format
					player.stream = newStream(format.SampleRate, streamer, player.volume, player.muted)
					player.status = playing
					speaker.Play(beep.Seq(player.stream.volume, beep.Callback(func() {
						player.event <- true
					})))
					drawer.updateTextData(&player)
				}
			case string:
				player.latestMessage = value.(string)
				drawer.reDrawMetaData(&player)
			}

		// handle tcell events
		case event := <-tcellEvent:
			switch event := event.(type) {
			case *tcell.EventResize:
				drawer.reDrawMetaData(&player)
			case *tcell.EventKey:
				if event.Key() == tcell.KeyESC {
					drawer.screen.Fini()
					player.stop()
					// crashes after suspend
					// speaker.Close()
					ticker.Stop()
					os.Exit(0)
				}

				if event.Key() != tcell.KeyRune {
					continue
				}

				switch event.Rune() {
				case ' ':
					if player.status == seekBWD || player.status == seekFWD {
						player.status = buf
					}
					if player.status == playing {
						player.status = paused
					} else if player.status == paused {
						player.status = playing
					} else {
						continue
					} // FIXME???
					speaker.Lock()
					player.stream.ctrl.Paused = !player.stream.ctrl.Paused
					speaker.Unlock()
				case 'a', 'A':
					if player.stream == nil || player.status == stopped {
						continue
					}
					if player.status != seekBWD && player.status != seekFWD {
						buf = player.status
						player.status = seekBWD
					}
					speaker.Lock()
					player.changePosition(0 - player.format.SampleRate.N(time.Second*2))
					player.currentPos = player.getCurrentTrackPosition()
					speaker.Unlock()

				case 'd', 'D':
					if player.stream == nil || player.status == stopped {
						continue
					}
					if player.status != seekFWD && player.status != seekBWD {
						buf = player.status
						player.status = seekFWD
					}
					speaker.Lock()
					player.changePosition(player.format.SampleRate.N(time.Second * 2))
					player.currentPos = player.getCurrentTrackPosition()
					speaker.Unlock()

				case 's', 'S':
					if player.stream == nil || player.volume < -9.6 {
						continue
					}
					player.volume -= 0.5
					speaker.Lock()
					if !player.muted && player.volume < -9.6 {
						player.muted = true
						player.stream.volume.Silent = player.muted
					}
					player.stream.volume.Volume = player.volume
					speaker.Unlock()

				case 'w', 'W':
					if player.stream == nil || player.volume > -0.4 {
						continue
					}
					player.volume += 0.5
					speaker.Lock()
					if player.muted {
						player.muted = false
						player.stream.volume.Silent = player.muted
					}
					player.stream.volume.Volume = player.volume
					speaker.Unlock()

				case 'm', 'M':
					if player.stream == nil {
						continue
					}
					player.muted = !player.muted
					speaker.Lock()
					player.stream.volume.Silent = player.muted
					speaker.Unlock()

				case 'r', 'R':
					player.playbackMode = (player.playbackMode + 1) % 5

				case 'b', 'B':
					player.skip(false)

				case 'f', 'F':
					player.skip(true)

				case 't', 'T':
					drawer.lightMode = !drawer.lightMode
					fgColor, bgColor, _ := drawer.style.Decompose()
					drawer.style = drawer.style.Foreground(bgColor).
						Background(fgColor)
					drawer.reDrawMetaData(&player)
				case 'i', 'I':
					drawer.artMode = (drawer.artMode + 1) % 6
					drawer.redrawArt(&player)

				// FIXME: hangs sometimes, not all deadlocks are resolved
				// probably poll events listener in other goroutine
				// trying to get events from screen that is disabled
				case 'o', 'O':
					drawer.screen.Fini()
					jsonString = handleInput("Enter new album link, leave empty to go back")
					if jsonString == "q" {
						player.stop()
						// crashes after suspend
						// speaker.Close()
						ticker.Stop()
						os.Exit(0)
					} else if jsonString != "" {
						player.stop()
						player.initPlayer(parseJSON(jsonString))
					}
					drawer.initScreen(&player)
					continue
				}
				drawer.updateTextData(&player)
			}
		}
	}
}
