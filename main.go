package main

import (
	"bufio"
	"bytes"
	"encoding/json"
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
	ByArtist      map[string]interface{} `json:"byArtist"`      // field "name" contains artist/band
	Name          string                 `json:"name"`          // album title
	DatePublished string                 `json:"datePublished"` // release date
	Image         string                 `json:"image"`         // link to album art
	Tracks        Track                  `json:"track"`         // container for track data
	Tags          []string               `json:"keywords"`      // tags/keywords
	AlbumArt      image.Image
}

type Track struct {
	NumberOfItems   int        `json:"numberOfItems"`   // tracks in album
	ItemListElement []ItemList `json:"itemListElement"` // further container for track data
}

type ItemList struct {
	TrackInfo Item `json:"item"`     // further container for track data
	Position  int  `json:"position"` // track number
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
)

func (status playbackStatus) String() string {
	return [3]string{"Stopped:", "Playing:", "Pause:"}[status]
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
	status       playbackStatus
	albumList    *Album // not a list at the moment
	stream       *mediaStream
	format       beep.Format

	playbackMode  playbackMode
	currentPos    string
	latestMessage string
	volume        float64
	muted         bool
	xOffset       int

	screen tcell.Screen
	style  tcell.Style

	event chan interface{}
}

func (player *playback) changePosition(pos int) {
	if player.status != playing {
		return
	}
	newPos := player.stream.streamer.Position()
	newPos += pos
	if newPos < 0 {
		newPos = 0
	}
	if newPos >= player.stream.streamer.Len() {
		newPos = player.stream.streamer.Len() - 1
	}
	if err := player.stream.streamer.Seek(newPos); err != nil {
		// probably everything goes wrong in case of an error
		player.latestMessage = err.Error()
	}
}

func (player *playback) newStream(sampleRate beep.SampleRate, streamer beep.StreamSeekCloser) *mediaStream {
	ctrl := &beep.Ctrl{Streamer: streamer}
	resampler := beep.Resample(4, 44100, sampleRate, ctrl)
	volume := &effects.Volume{Streamer: resampler, Base: 2, Volume: float64(player.volume), Silent: player.muted}
	return &mediaStream{sampleRate, streamer, ctrl, resampler, volume}
}

func (player *playback) getNewTrack() {
	if len(player.albumList.Tracks.ItemListElement) == 0 {
		player.latestMessage = "No album data was found"
		return
	}
	trackNumber := player.currentTrack
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
				reportError(err)
			}
			if response.StatusCode > 299 {
				fmt.Printf("Request failed with status code: %d\n", response.StatusCode)
				os.Exit(1)
			}
			player.latestMessage = fmt.Sprint(filename, " - ", response.Status, " Downloading...")

			cachedResponses[trackNumber], err = io.ReadAll(response.Body)
			if err != nil {
				player.latestMessage = err.Error()
				return
			}
			defer response.Body.Close()
			player.latestMessage = fmt.Sprint(filename, " - Done")
			player.event <- trackNumber
			return
		}
	}
	player.latestMessage = "Track is currently not available for streaming"
}

func (player *playback) getCurrentTrackPosition() string {
	return fmt.Sprint(player.format.SampleRate.D(player.stream.
		streamer.Position()).Round(time.Second))
}

func (player *playback) skip(forward bool) {
	if player.playbackMode == random {
		player.nextTrack()
		return
	}
	player.stop()
	if forward {
		player.currentTrack = (player.currentTrack + 1) % player.albumList.Tracks.NumberOfItems
	} else {
		player.currentTrack = (player.albumList.Tracks.NumberOfItems + player.currentTrack - 1) % player.albumList.Tracks.NumberOfItems
	}
	go player.getNewTrack()
}

func (player *playback) nextTrack() {
	player.stop()
	switch player.playbackMode {
	case random:
		rand.Seed(time.Now().UnixNano())
		player.currentTrack = rand.Intn(player.albumList.Tracks.NumberOfItems)
		go player.getNewTrack()
	case repeatOne:
		go player.getNewTrack()
	case repeat:
		player.skip(true)
	case normal:
		if player.currentTrack == player.albumList.Tracks.NumberOfItems-1 {
			player.status = stopped
			return
		}
		player.skip(true)
	}
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

func (player *playback) reDrawMetaData(x, y int) {
	player.screen.Fill(' ', player.style)

	// draw album art
	options := convert.Options{
		Ratio:           1.0,
		FixedWidth:      -1,
		FixedHeight:     -1,
		FitScreen:       true,
		StretchedScreen: false,
		Colored:         true,
		Reversed:        false,
	}
	art := convert.NewImageConverter().Image2CharPixelMatrix(player.albumList.AlbumArt, &options)
	xReset := x
	for _, pixelY := range art {
		for _, pixelX := range pixelY {
			player.screen.SetContent(x, y, rune(pixelX.Char), nil,
				player.style.Foreground(tcell.NewRGBColor(int32(pixelX.R),
					int32(pixelX.G), int32(pixelX.B))))
			x++
		}
		x = xReset
		y++
	}
	player.xOffset = len(art[0]) + 6
	player.updateTextData()
	player.screen.Sync()
}

func (player *playback) updateTextData() {

	_, height := player.screen.Size()

	player.clearString(player.xOffset, height-2)
	player.drawString(player.xOffset, height-2, player.latestMessage)

	var sbuilder strings.Builder
	if len(player.albumList.Tracks.ItemListElement) == 0 {
		player.screen.Show()
		return
	}
	item := player.albumList.Tracks.ItemListElement[player.currentTrack]
	player.clearString(player.xOffset, 1)
	player.drawString(player.xOffset, 1, player.status.String())

	fmt.Fprintf(&sbuilder, "%d/%d - %s", item.Position,
		player.albumList.Tracks.NumberOfItems, item.TrackInfo.Name)
	player.clearString(player.xOffset, 2)
	player.drawString(player.xOffset, 2, sbuilder.String())
	sbuilder.Reset()

	fmt.Fprintf(&sbuilder, "Artist: %s", player.albumList.ByArtist["name"])
	player.clearString(player.xOffset, 3)
	player.drawString(player.xOffset, 3, sbuilder.String())
	sbuilder.Reset()

	fmt.Fprintf(&sbuilder, "Album: %s", player.albumList.Name)
	player.clearString(player.xOffset, 4)
	player.drawString(player.xOffset, 4, sbuilder.String())
	sbuilder.Reset()

	fmt.Fprintf(&sbuilder, "Release Date: %s", player.albumList.DatePublished[:11])
	player.clearString(player.xOffset, 5)
	player.drawString(player.xOffset, 5, sbuilder.String())
	sbuilder.Reset()

	fmt.Fprintf(&sbuilder, "Mode: %s", player.playbackMode.String())
	player.clearString(player.xOffset, 6)
	player.drawString(player.xOffset, 6, sbuilder.String())
	sbuilder.Reset()

	if player.muted {
		fmt.Fprintf(&sbuilder, "Volume: Mute")
	} else {
		fmt.Fprintf(&sbuilder, "Volume: %.0f%%", (100 + player.volume*10))
	}
	player.clearString(player.xOffset, 7)
	player.drawString(player.xOffset, 7, sbuilder.String())
	sbuilder.Reset()

	var seconds float64
	for _, value := range item.TrackInfo.AdditionalProperty {
		if value.Name == "duration_secs" {
			seconds = value.Value.(float64)
		}
	}

	fmt.Fprintf(&sbuilder, "%s/%s", player.currentPos,
		time.Duration(seconds*float64(time.Second)).Round(time.Second))
	player.clearString(player.xOffset, 9)
	player.drawString(player.xOffset, 9, sbuilder.String())
	sbuilder.Reset()

	fmt.Fprint(&sbuilder, player.albumList.Tags)
	player.clearString(player.xOffset, 10)
	player.drawString(player.xOffset, 10, sbuilder.String())
	sbuilder.Reset()

	player.screen.Show()
}

func (player *playback) drawString(x, y int, str string) {
	for _, r := range str {
		player.screen.SetContent(x, y, r, nil, player.style)
		x++
	}
}

func (player *playback) clearString(x, y int) {
	width, _ := player.screen.Size()
	for i := x; i < width; i++ {
		player.screen.SetContent(i, y, ' ', nil, player.style)
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

func parseJSON(input string) *Album {
	var album Album
	response, err := http.DefaultClient.Do(createNewRequest(input))
	if err != nil {
		reportError(err)
	}
	if response.StatusCode > 299 {
		fmt.Printf("Request failed with status code: %d\n", response.StatusCode)
		os.Exit(1)
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
	response.Body.Close()
	reader := bytes.NewBuffer(body)

	var jsonstring string
	for {
		jsonstring, err = reader.ReadString('\n')
		if err != nil {
			reportError(err)
		}
		if strings.Contains(jsonstring, "application/ld+json") {
			jsonstring, err = reader.ReadString('\n')
			if err != nil {
				reportError(err)
			}
			break
		}
	}
	// TODO: track and album pages are different, for now only album pages supported
	err = json.Unmarshal([]byte(jsonstring), &album)
	if err != nil {
		reportError(err)
	}
	return &album
}

func initScreen() (screen tcell.Screen, style tcell.Style) {
	screen, err := tcell.NewScreen()
	reportError(err)
	err = screen.Init()
	reportError(err)
	style = tcell.StyleDefault.
		Background(tcell.NewHexColor(0x2B2B2B))
	screen.Fill(' ', style)
	screen.Show()
	return screen, style
}

func (player *playback) initPlayer(input string) {
	cachedResponses = make(map[int][]byte)
	// keeps volume and playback mode from previous playback
	*player = playback{playbackMode: player.playbackMode, volume: player.volume,
		muted: player.muted}
	player.albumList = parseJSON(input)
	player.albumList.AlbumArt = image.NewRGBA(image.Rect(0, 0, 1, 1))
	player.event = make(chan interface{})

	go player.downloadCover()
	go player.getNewTrack()
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
	stdinReader := bufio.NewReader(os.Stdin)
	fmt.Println("Enter bandcamp album link:")
	input, err := stdinReader.ReadString('\n')
	if err != nil {
		reportError(err)
	}
	player.initPlayer(strings.Trim(input, "\n"))
	player.screen, player.style = initScreen()
	player.reDrawMetaData(3, 0)
	ticker := time.NewTicker(time.Second)
	timer := ticker.C
	tcellEvent := make(chan tcell.Event)

	// doesn't fail without screen and continues to work
	// after reinitialization normally
	// does it even end?
	go func() {
		for {
			tcellEvent <- player.screen.PollEvent()
		}
	}()

	// event loop
	for {
		select {
		case <-timer:
			if player.format.SampleRate != 0 {
				speaker.Lock()
				player.currentPos = player.getCurrentTrackPosition()
				speaker.Unlock()
				player.updateTextData()
			}

		// handle player events
		case value := <-player.event:
			switch value.(type) {
			case bool:
				player.nextTrack()
				player.updateTextData()
			case int:
				if value.(int) == player.currentTrack && player.status != playing {
					streamer, format, err := mp3.Decode(&bytesRSC{bytes.
						NewReader(cachedResponses[value.(int)])})
					if err != nil {
						player.latestMessage = fmt.Sprint(err)
						continue
					}
					player.format = format
					player.stream = player.newStream(format.SampleRate, streamer)
					player.status = playing
					speaker.Play(beep.Seq(player.stream.volume, beep.Callback(func() {
						player.event <- true
					})))
				}
			case string:
				player.latestMessage = value.(string)
				player.reDrawMetaData(3, 0)
			}

		// handle tcell events
		case event := <-tcellEvent:
			switch event := event.(type) {
			case *tcell.EventResize:
				player.reDrawMetaData(3, 0)
			case *tcell.EventKey:
				if event.Key() == tcell.KeyESC {
					player.stop()
					player.screen.Fini()
					speaker.Close()
					ticker.Stop()
					os.Exit(0)
				}

				if event.Key() != tcell.KeyRune {
					continue
				}

				switch event.Rune() {
				case ' ':
					if player.stream == nil || player.status == stopped {
						continue
					}
					if player.status == playing {
						player.status = paused
					} else {
						player.status = playing
					}
					speaker.Lock()
					player.stream.ctrl.Paused = !player.stream.ctrl.Paused
					speaker.Unlock()
				case 'a', 'A':
					if player.stream == nil {
						continue
					}
					speaker.Lock()
					player.changePosition(0 - player.format.SampleRate.N(time.Second*2))
					player.currentPos = player.getCurrentTrackPosition()
					speaker.Unlock()

				case 'd', 'D':
					if player.stream == nil {
						continue
					}
					speaker.Lock()
					player.changePosition(player.format.SampleRate.N(time.Second * 2))
					player.currentPos = player.getCurrentTrackPosition()
					speaker.Unlock()

				case 's', 'S':
					if player.volume < -9.6 || player.stream == nil {
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
					if player.volume > -0.4 || player.stream == nil {
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
					player.playbackMode = (player.playbackMode + 1) % 4

				case 'b', 'B':
					player.skip(false)

				case 'f', 'F':
					player.skip(true)
				case 'o', 'O':
					player.screen.Fini()
					fmt.Println("Enter another album link, or leave empty to go back:")
					input, err := stdinReader.ReadString('\n')
					if err != nil {
						reportError(err)
					}
					if input != "\n" {
						player.stop()
						player.initPlayer(strings.Trim(input, "\n"))
						player.screen, player.style = initScreen()
						player.reDrawMetaData(3, 0)
						continue
					}
					player.screen, player.style = initScreen()
				}
				player.updateTextData()
			}
		}
	}
}
