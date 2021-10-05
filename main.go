package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"time"
	"unicode"

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
	Duration           string     `json:"duration"`           // string representation of dureation P##H##M##S:
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

func (player *playback) changePosition(pos int) {
	if player.Status != playing {
		return
	}
	newPos := player.Stream.streamer.Position()
	newPos += pos
	if newPos < 0 {
		newPos = 0
	}
	if newPos >= player.Stream.streamer.Len() {
		newPos = player.Stream.streamer.Len() - 1
	}
	if err := player.Stream.streamer.Seek(newPos); err != nil {
		player.LatestMessage = err.Error()
	}
}

func (player *playback) newStream(sampleRate beep.SampleRate, streamer beep.StreamSeekCloser) *mediaStream {
	ctrl := &beep.Ctrl{Streamer: streamer}
	resampler := beep.Resample(4, 44100, sampleRate, ctrl)
	volume := &effects.Volume{Streamer: resampler, Base: 2, Volume: float64(player.volume), Silent: player.muted}
	return &mediaStream{sampleRate, streamer, ctrl, resampler, volume}
}

var cachedResponses map[int][]byte

type playback struct {
	CurrentTrack int
	Status       playbackStatus
	AlbumList    Album // not a list at the moment
	Stream       *mediaStream
	Format       beep.Format

	PlaybackMode  playbackMode
	CurrentPos    string
	LatestMessage string
	volume        float64
	muted         bool
	xOffset       int

	timer       <-chan time.Time
	next        chan bool
	trackNumber chan int
}

func (player *playback) getNewTrack() {
	if len(player.AlbumList.Tracks.ItemListElement) == 0 {
		player.LatestMessage = "No album data was found"
		return
	}
	trackNumber := player.CurrentTrack
	item := player.AlbumList.Tracks.ItemListElement[trackNumber]
	filename := fmt.Sprint(player.AlbumList.ByArtist["name"], " - ", item.TrackInfo.Name, ".mp3")

	if cachedResponses[trackNumber] != nil {
		player.LatestMessage = fmt.Sprint(filename, " - Cached")
		player.trackNumber <- trackNumber
		return
	}

	player.LatestMessage = fmt.Sprint(filename, " - Fetching...")
	for _, value := range item.TrackInfo.AdditionalProperty {
		// not all tracks are available for streaming,
		// there is a `streaming` field in JSON
		// but tracks that haven't been published yet
		// (pre order albums with some tracks available
		// for streaming) don't have it at all
		if value.Name == "file_mp3-128" {
			request, err := http.NewRequest("GET", value.Value.(string), nil)
			if err != nil {
				reportError(err)
			}
			request.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/93.0.4577.82 Safari/537.36")
			response, err := http.DefaultClient.Do(request)
			if err != nil {
				reportError(err)
			}
			if response.StatusCode > 299 {
				fmt.Printf("Request failed with status code: %d\n", response.StatusCode)
				os.Exit(1)
			}
			player.LatestMessage = fmt.Sprint(filename, " - ", response.Status, " Downloading...")

			cachedResponses[trackNumber], err = io.ReadAll(response.Body)
			if err != nil {
				player.LatestMessage = err.Error()
				return
			}
			defer response.Body.Close()
			player.LatestMessage = fmt.Sprint(filename, " - Done")
			player.trackNumber <- trackNumber
			return
		}
	}
	player.LatestMessage = "Track is currently not available for streaming"
}

func (player *playback) getCurrentTrackPosition() string {
	return fmt.Sprint(player.Format.SampleRate.D(player.Stream.streamer.Position()).Round(time.Second))
}

func (player *playback) skip(forward bool) {
	if player.PlaybackMode == random {
		player.nextTrack()
		return
	}
	player.stop()
	if forward {
		player.CurrentTrack = (player.CurrentTrack + 1) % player.AlbumList.Tracks.NumberOfItems
	} else {
		player.CurrentTrack = (player.AlbumList.Tracks.NumberOfItems + player.CurrentTrack - 1) % player.AlbumList.Tracks.NumberOfItems
	}
	go player.getNewTrack()
}

func (player *playback) nextTrack() {
	player.stop()
	switch player.PlaybackMode {
	case random:
		rand.Seed(time.Now().UnixNano())
		player.CurrentTrack = rand.Intn(player.AlbumList.Tracks.NumberOfItems)
		go player.getNewTrack()
	case repeatOne:
		go player.getNewTrack()
	case repeat:
		player.skip(true)
	case normal:
		if player.CurrentTrack == player.AlbumList.Tracks.NumberOfItems-1 {
			player.Status = stopped
			return
		}
		player.skip(true)
	}
}

func (player *playback) stop() {
	speaker.Clear()
	player.Status = stopped
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

func (player *playback) reDrawMetaData(screen tcell.Screen, x, y int, style tcell.Style) {
	screen.Fill(' ', style)

	// draw album art
	art := convert.NewImageConverter().Image2CharPixelMatrix(player.AlbumList.AlbumArt, &convert.DefaultOptions)
	xReset := x
	for _, pixelY := range art {
		for _, pixelX := range pixelY {
			screen.SetContent(x, y, rune(pixelX.Char), nil, style.Foreground(tcell.NewRGBColor(int32(pixelX.R), int32(pixelX.G), int32(pixelX.B))))
			x++
		}
		x = xReset
		y++
	}

	player.xOffset = len(art[0]) + 6
	player.updateTextData(screen, style)
	screen.Sync()
}

func (player *playback) updateTextData(screen tcell.Screen, style tcell.Style) {
	clearString(screen, player.xOffset, 11, style)
	drawString(screen, player.xOffset, 11, player.LatestMessage, style)

	var sbuilder strings.Builder
	if len(player.AlbumList.Tracks.ItemListElement) == 0 {
		screen.Show()
		return
	}
	item := player.AlbumList.Tracks.ItemListElement[player.CurrentTrack]
	clearString(screen, player.xOffset, 1, style)
	drawString(screen, player.xOffset, 1, player.Status.String(), style)

	fmt.Fprintf(&sbuilder, "%d/%d - %s", item.Position,
		player.AlbumList.Tracks.NumberOfItems, item.TrackInfo.Name)
	clearString(screen, player.xOffset, 2, style)
	drawString(screen, player.xOffset, 2, sbuilder.String(), style)
	sbuilder.Reset()

	fmt.Fprintf(&sbuilder, "Artist: %s", player.AlbumList.ByArtist["name"])
	clearString(screen, player.xOffset, 3, style)
	drawString(screen, player.xOffset, 3, sbuilder.String(), style)
	sbuilder.Reset()

	fmt.Fprintf(&sbuilder, "Album: %s", player.AlbumList.Name)
	clearString(screen, player.xOffset, 4, style)
	drawString(screen, player.xOffset, 4, sbuilder.String(), style)
	sbuilder.Reset()

	fmt.Fprintf(&sbuilder, "Release Date: %s", player.AlbumList.DatePublished[:11])
	clearString(screen, player.xOffset, 5, style)
	drawString(screen, player.xOffset, 5, sbuilder.String(), style)
	sbuilder.Reset()

	fmt.Fprintf(&sbuilder, "Mode: %s", player.PlaybackMode.String())
	clearString(screen, player.xOffset, 6, style)
	drawString(screen, player.xOffset, 6, sbuilder.String(), style)
	sbuilder.Reset()

	if player.muted {
		fmt.Fprintf(&sbuilder, "Volume: Mute")
	} else {
		fmt.Fprintf(&sbuilder, "Volume: %.0f%%", (100 + player.volume*10))
	}
	clearString(screen, player.xOffset, 7, style)
	drawString(screen, player.xOffset, 7, sbuilder.String(), style)
	sbuilder.Reset()

	var seconds float64
	for _, value := range item.TrackInfo.AdditionalProperty {
		if value.Name == "duration_secs" {
			seconds = value.Value.(float64)
		}
	}

	fmt.Fprintf(&sbuilder, "%s/%s", player.CurrentPos,
		time.Duration(seconds*float64(time.Second)).Round(time.Second))
	clearString(screen, player.xOffset, 9, style)
	drawString(screen, player.xOffset, 9, sbuilder.String(), style)
	sbuilder.Reset()

	clearString(screen, player.xOffset, 11, style)
	drawString(screen, player.xOffset, 11, player.LatestMessage, style)

	screen.Show()
}

func drawString(screen tcell.Screen, x, y int, str string, style tcell.Style) {
	for _, r := range str {
		screen.SetContent(x, y, r, nil, style)
		x++
	}
}

func clearString(screen tcell.Screen, x, y int, style tcell.Style) {
	width, _ := screen.Size()
	for i := x; i < width; i++ {
		screen.SetContent(i, y, ' ', nil, style)
	}
}

func reportError(err error) {
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func parseJSON(input string) (album Album) {
	// TODO: link validation, note: not all bandcamp artist are hosted on whatever.bandcamp.com
	// apparently band pages work, if they have album on main page
	request, err := http.NewRequest("GET", input, nil)
	if err != nil {
		// TODO: instead of simply crashing on every step, it would be nice to ask for a proper link/explain problem
		reportError(err)
	}

	request.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/93.0.4577.82 Safari/537.36")
	// set mobile view, html weights a bit less than desktop version
	request.Header.Set("Cookie", "mvp=p")

	// make request for html page
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		reportError(err)
	}
	body, err := io.ReadAll(response.Body)
	if response.StatusCode > 299 {
		fmt.Printf("Request failed with status code: %d\n", response.StatusCode)
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
	// track pages will likely crash
	err = json.Unmarshal([]byte(jsonstring), &album)
	if err != nil {
		reportError(err)
	}

	request, err = http.NewRequest("GET", album.Image, nil)
	if err != nil {
		reportError(err)
	}
	request.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/93.0.4577.82 Safari/537.36")
	response, err = http.DefaultClient.Do(request)
	if err != nil {
		fmt.Println(err, "trying http://")
	}
	request, err = http.NewRequest("GET", strings.Replace(album.Image, "https://", "http://", 1), nil)
	if err != nil {
		reportError(err)
	}
	request.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/93.0.4577.82 Safari/537.36")
	response, err = http.DefaultClient.Do(request)
	if err != nil {
		reportError(err)
	}
	defer response.Body.Close()
	// TODO: add default case that generates white image
	// is there anything other than jpeg?
	switch response.Header.Get("Content-Type") {
	case "image/jpeg":
		album.AlbumArt, err = jpeg.Decode(response.Body)
	case "image/png":
		album.AlbumArt, err = png.Decode(response.Body)
	}
	return album
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
	player.AlbumList = parseJSON(strings.Trim(input, "\n"))
	player.timer = time.Tick(time.Second)
	player.next = make(chan bool)
	player.trackNumber = make(chan int)
	go player.getNewTrack()
}

func main() {
	var player playback
	player = playback{PlaybackMode: player.PlaybackMode, volume: player.volume, muted: player.muted}
	// FIXME: behaves weird after coming from suspend (high CPU load)
	// FIXME: device does not initialize after suspend
	// FIXME: takes device to itself, doesn't allow any other program to use it, and can't use it, if device is already being used
	stdinReader := bufio.NewReader(os.Stdin)
	fmt.Print("Enter bandcamp album link: ")
	input, err := stdinReader.ReadString('\n')
	if err != nil {
		reportError(err)
	}
	player.initPlayer(input)
	screen, style := initScreen()
	player.reDrawMetaData(screen, 3, 0, style)
	events := make(chan tcell.Event)
	// FIXME: ?
	go func() {
		for {
			events <- screen.PollEvent()
		}
	}()

	// event loop
	for {
		select {
		case <-player.timer:
			if player.Format.SampleRate != 0 {
				speaker.Lock()
				player.CurrentPos = player.getCurrentTrackPosition()
				speaker.Unlock()
			}
			player.updateTextData(screen, style)
		case <-player.next:
			player.nextTrack()
			player.updateTextData(screen, style)
		case trackNumber := <-player.trackNumber:
			if trackNumber == player.CurrentTrack && player.Status != playing {
				streamer, format, err := mp3.Decode(&bytesRSC{bytes.
					NewReader(cachedResponses[trackNumber])})
				if err != nil {
					player.LatestMessage = fmt.Sprint(err)
				}
				player.Format = format
				player.Stream = player.newStream(format.SampleRate, streamer)
				player.Status = playing
				speaker.Play(beep.Seq(player.Stream.volume, beep.Callback(func() {
					player.next <- true
				})))
			}
		case event := <-events:
			switch event := event.(type) {
			case *tcell.EventResize:
				player.reDrawMetaData(screen, 3, 0, style)
			case *tcell.EventKey:
				if event.Key() == tcell.KeyESC {
					screen.Fini()
					os.Exit(0)
				}

				if event.Key() != tcell.KeyRune {
					continue
				}

				switch unicode.ToLower(event.Rune()) {
				case ' ':
					if player.Stream == nil || player.Status == stopped {
						continue
					}
					if player.Status == playing {
						player.Status = paused
					} else {
						player.Status = playing
					}
					speaker.Lock()
					player.Stream.ctrl.Paused = !player.Stream.ctrl.Paused
					speaker.Unlock()
				case 'a':
					if player.Stream == nil {
						continue
					}
					speaker.Lock()
					player.changePosition(0 - player.Format.SampleRate.N(time.Second*2))
					player.CurrentPos = player.getCurrentTrackPosition()
					speaker.Unlock()

				case 'd':
					if player.Stream == nil {
						continue
					}
					speaker.Lock()
					player.changePosition(player.Format.SampleRate.N(time.Second * 2))
					player.CurrentPos = player.getCurrentTrackPosition()
					speaker.Unlock()

				case 's':
					if player.volume < -9.6 || player.Stream == nil {
						continue
					}
					player.volume -= 0.5
					speaker.Lock()
					if !player.muted && player.volume < -9.6 {
						player.muted = true
						player.Stream.volume.Silent = player.muted
					}
					player.Stream.volume.Volume = player.volume
					speaker.Unlock()

				case 'w':
					if player.volume > -0.4 || player.Stream == nil {
						continue
					}
					player.volume += 0.5
					speaker.Lock()
					if player.muted {
						player.muted = false
						player.Stream.volume.Silent = player.muted
					}
					player.Stream.volume.Volume = player.volume
					speaker.Unlock()

				case 'm':
					if player.Stream == nil {
						continue
					}
					player.muted = !player.muted
					speaker.Lock()
					player.Stream.volume.Silent = player.muted
					speaker.Unlock()

				case 'r':
					player.PlaybackMode = (player.PlaybackMode + 1) % 4

				case 'b':
					player.skip(false)

				case 'f':
					player.skip(true)
				case 'o':
					screen.Fini()
					fmt.Println("Enter another album link, or leave empty to go back:")
					input, err := stdinReader.ReadString('\n')
					if err != nil {
						reportError(err)
					}
					if input != "\n" {
						player.stop()
						player = playback{PlaybackMode: player.PlaybackMode, volume: player.volume, muted: player.muted}
						player.initPlayer(input)
						screen, style = initScreen()
						player.reDrawMetaData(screen, 3, 0, style)
						continue
					}
					screen, style = initScreen()
				}
				player.updateTextData(screen, style)
			}
		}
	}
}

func init() {
	sr := beep.SampleRate(44100)
	speaker.Init(sr, sr.N(time.Second/10))
}
