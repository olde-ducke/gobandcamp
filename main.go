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
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/faiface/beep"
	"github.com/faiface/beep/effects"
	"github.com/faiface/beep/mp3"
	"github.com/faiface/beep/speaker"
	"github.com/qeesung/image2ascii/convert"
)

// TODO: struct names and fields are complete mess
type Album struct {
	ByArtist      map[string]interface{} `json:"byArtist"`      // field "name" contains artist/band
	Name          string                 `json:"name"`          // album title
	DatePublished string                 `json:"datePublished"` // release date
	Image         string                 `json:"image"`         // link to album art
	Tracks        Track                  `json:"track"`         // container for track data
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

type playerStatus int

const (
	playing playerStatus = iota
	stopped
	paused
)

func (status playerStatus) String() string {
	return [3]string{"Playing:", "Playback Stopped:", "Pause:"}[status]
}

type mediaStream struct {
	sampleRate beep.SampleRate
	streamer   beep.StreamSeekCloser
	ctrl       *beep.Ctrl
	resampler  *beep.Resampler
	volume     *effects.Volume
}

func (stream *mediaStream) changePosition(pos int) {
	newPos := stream.streamer.Position()
	newPos += pos
	if newPos < 0 {
		newPos = 0
	}
	if newPos >= stream.streamer.Len() {
		newPos = stream.streamer.Len() - 1
	}
	if err := stream.streamer.Seek(newPos); err != nil {
		log.Println(err)
	}
}

func newStream(sampleRate beep.SampleRate, streamer beep.StreamSeekCloser) *mediaStream {
	ctrl := &beep.Ctrl{Streamer: streamer}
	resampler := beep.Resample(4, 44100, sampleRate, ctrl)
	volume := &effects.Volume{Streamer: resampler, Base: 2}
	return &mediaStream{sampleRate, streamer, ctrl, resampler, volume}
}

type playback struct {
	CurrentTrack  int
	LatestMessage string
	Status        playerStatus
	AlbumList     Album // not a list at the moment
	CurrentPos    string
	AlbumArt      image.Image
	PlaybackMode  playbackMode
	Stream        *mediaStream
	Format        beep.Format

	timer <-chan time.Time
	next  chan bool
}

func (player *playback) skip(forward bool) {
	if player.PlaybackMode == random {
		player.nextTrack()
		return
	}
	if forward {
		player.CurrentTrack = (player.CurrentTrack + 1) % player.AlbumList.Tracks.NumberOfItems
	} else {
		player.CurrentTrack = (player.AlbumList.Tracks.NumberOfItems + player.CurrentTrack - 1) % player.AlbumList.Tracks.NumberOfItems
	}
	go player.startNewStream()
}

func (player *playback) nextTrack() {
	switch player.PlaybackMode {
	case random:
		rand.Seed(time.Now().UnixNano())
		player.CurrentTrack = rand.Intn(player.AlbumList.Tracks.NumberOfItems)
		go player.startNewStream()
	case repeatOne:
		go player.startNewStream()
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

func (player *playback) startNewStream() {
	speaker.Clear()
	player.CurrentPos = "0s"
	player.Status = stopped

	item := player.AlbumList.Tracks.ItemListElement[player.CurrentTrack]
	player.LatestMessage = fmt.Sprint(player.AlbumList.ByArtist["name"], " - ", item.TrackInfo.Name, ".mp3 - fetching...")
	for _, value := range item.TrackInfo.AdditionalProperty {
		// hardcoded JSON field name
		if value.Name == "file_mp3-128" {

			// TODO: not all tracks are streamable on service, pretty sure there was "streamable" field in JSON
			// new request to media server
			request, err := http.NewRequest("GET", value.Value.(string), nil)
			if err != nil {
				log.Fatal(err)
			}
			request.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/93.0.4577.82 Safari/537.36")
			response, err := http.DefaultClient.Do(request)
			if err != nil {
				log.Fatal(err)
			}
			if response.StatusCode > 299 {
				log.Fatalf("Request failed with status code: %d\n", response.StatusCode)
			}
			player.LatestMessage = fmt.Sprint(player.AlbumList.ByArtist["name"], " - ", item.TrackInfo.Name, ".mp3 - ",
				response.Status, " Downloading...")

			body, err := io.ReadAll(response.Body)
			if err != nil {
				player.LatestMessage = err.Error()
				return
			}
			buffer := &cachedBytes{bytes.NewReader(body)}
			defer response.Body.Close()
			player.LatestMessage = "Done"

			streamer, format, err := mp3.Decode(buffer)
			if err != nil {
				player.LatestMessage = fmt.Sprint(err)
			}

			speaker.Lock()
			player.Format = format
			player.Stream = newStream(format.SampleRate, streamer)
			player.Status = playing
			speaker.Unlock()
			speaker.Play(beep.Seq(player.Stream.volume, beep.Callback(func() {
				player.next <- true
			})))
		}
	}
}

// response.Body doesn't implement Seek() method
// beep isn't bothered by this, but trying to
// call Seek() will fail since Len() will always return 0
// by using bytes.Reader and implementing empty Close() method
// we get io.ReadSeekCloser, which satisfies requirements of beep streamers
// (needs ReadCloser) and implements Seek() method
type cachedBytes struct {
	*bytes.Reader
}

func (c cachedBytes) Close() error {
	return nil
}

func main() {
	var player playback

	stdinReader := bufio.NewReader(os.Stdin)
	fmt.Print("Enter bandcamp album link: ")
	input, err := stdinReader.ReadString('\n')
	if err != nil {
		log.Fatal(err)
	}
	input = strings.Trim(input, "\n")

	// TODO: link validation, note: not all bandcamp artist are hosted on whatever.bandcamp.com
	// apparently band pages work, if they have album on main page
	request, err := http.NewRequest("GET", input, nil)
	if err != nil {
		// TODO: instead of simply crashing on every step, it would be nice to ask for a proper link/explain problem
		log.Fatal(err)
	}

	request.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/93.0.4577.82 Safari/537.36")
	// set mobile view, html weights a bit less than desktop version
	request.Header.Set("Cookie", "mvp=p")

	// make request for html page
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		log.Fatal(err)
	}
	body, err := io.ReadAll(response.Body)
	if response.StatusCode > 299 {
		log.Fatalf("Request failed with status code: %d\n", response.StatusCode)
	}
	player.LatestMessage = fmt.Sprint(input, " ", response.Status)
	response.Body.Close()

	reader := bytes.NewBuffer(body)

	var jsonstring string
	for {
		jsonstring, err = reader.ReadString('\n')
		if err != nil {
			log.Fatal(err)
		}
		if strings.Contains(jsonstring, "application/ld+json") {
			jsonstring, err = reader.ReadString('\n')
			if err != nil {
				log.Fatal(err)
			}
			break
		}
	}

	// TODO: track and album pages are different, for now only album pages supported
	// track pages will crash
	err = json.Unmarshal([]byte(jsonstring), &player.AlbumList)
	if err != nil {
		log.Fatal(err)
	}

	request, err = http.NewRequest("GET", player.AlbumList.Image, nil)
	if err != nil {
		log.Fatal(err)
	}
	request.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/93.0.4577.82 Safari/537.36")
	response, err = http.DefaultClient.Do(request)
	if err != nil {
		log.Println(err, "trying http://")
	}
	request, err = http.NewRequest("GET", strings.Replace(player.AlbumList.Image, "https://", "http://", 1), nil)
	if err != nil {
		log.Fatalln(err)
	}
	response, err = http.DefaultClient.Do(request)

	// TODO: add default case that generates white image
	// is there anything other than jpeg?
	switch response.Header.Get("Content-Type") {
	case "image/jpeg":
		player.AlbumArt, err = jpeg.Decode(response.Body)
	case "image/png":
		player.AlbumArt, err = png.Decode(response.Body)
	}

	// FIXME: behaves weird after coming from suspend (high CPU load)
	// FIXME: device does not initialize after suspend
	// FIXME: takes device to itself, doesn't allow any other program to use it, and can't use it, if device is already being used

	// get first stream, default value for current track is 0
	// start timer and set channel for sending end of track signal
	player.timer = time.Tick(time.Second)
	player.next = make(chan bool)
	go player.startNewStream()

loop:
	for {
		select {
		case <-player.timer:
			if player.Format.SampleRate != 0 {
				speaker.Lock()
				player.CurrentPos = fmt.Sprint(player.Format.SampleRate.D(player.Stream.streamer.Position()).Round(time.Second))
				speaker.Unlock()
			}
			player.printMetadata()
		case <-player.next:
			player.nextTrack()
		}

		switch strings.Trim(input, "\n") {
		case "m", "M":
			speaker.Lock()
			player.Stream.volume.Silent = !player.Stream.volume.Silent
			speaker.Unlock()
		//TODO: set volume limits
		case "s", "S":
			speaker.Lock()
			player.Stream.volume.Volume -= 0.5
			speaker.Unlock()
		case "w", "W":
			speaker.Lock()
			player.Stream.volume.Volume += 0.5
			speaker.Unlock()
		case "a", "A":
			speaker.Lock()
			player.Stream.changePosition(0 - player.Format.SampleRate.N(time.Second*2))
			speaker.Unlock()
		case "d", "D":
			speaker.Lock()
			player.Stream.changePosition(player.Format.SampleRate.N(time.Second * 2))
			speaker.Unlock()
		case "p", "P":
			if player.Status == playing {
				player.Status = paused
			} else {
				player.Status = playing
			}
			speaker.Lock()
			player.Stream.ctrl.Paused = !player.Stream.ctrl.Paused
			speaker.Unlock()
		case "f", "F":
			player.skip(true)
		case "b", "B":
			player.skip(false)
		case "r", "R":
			player.PlaybackMode = (player.PlaybackMode + 1) % 4
		case "q", "Q":
			speaker.Clear()
			clearScreen()
			break loop
		}
	}
}

func init() {
	sr := beep.SampleRate(44100)
	speaker.Init(sr, sr.N(time.Second/10))
}

func clearScreen() {
	cmd := exec.Command("clear")
	cmd.Stdout = os.Stdout
	cmd.Run()
}

func (player *playback) printMetadata() {
	clearScreen()

	item := player.AlbumList.Tracks.ItemListElement[player.CurrentTrack]
	//fmt.Printf("Lyrics:\n%s\n", item.TrackInfo.RecordingOf.Lyrics["text"])
	var seconds float64
	for _, value := range item.TrackInfo.AdditionalProperty {
		if value.Name == "duration_secs" {
			seconds = value.Value.(float64)
		}
	}
	art := strings.Split(convert.NewImageConverter().Image2ASCIIString(player.AlbumArt,
		&convert.DefaultOptions), "\n")

	out := make([]string, 12)

	var sbuilder strings.Builder
	out[1] = player.Status.String()

	fmt.Fprintf(&sbuilder, "%d/%d %s", item.Position,
		player.AlbumList.Tracks.NumberOfItems, item.TrackInfo.Name)
	out[2] = sbuilder.String()
	sbuilder.Reset()

	fmt.Fprintf(&sbuilder, "Artist: %s", player.AlbumList.ByArtist["name"])
	out[3] = sbuilder.String()
	sbuilder.Reset()

	fmt.Fprintf(&sbuilder, "Album: %s", player.AlbumList.Name)
	out[4] = sbuilder.String()
	sbuilder.Reset()

	fmt.Fprintf(&sbuilder, "Release Date: %s", player.AlbumList.DatePublished[:11])
	out[5] = sbuilder.String()
	sbuilder.Reset()

	fmt.Fprintf(&sbuilder, "Mode: %s", player.PlaybackMode.String())
	out[6] = sbuilder.String()
	sbuilder.Reset()

	fmt.Fprintf(&sbuilder, "%s/%s", player.CurrentPos,
		time.Duration(seconds*float64(time.Second)).Round(time.Second))
	out[9] = sbuilder.String()
	sbuilder.Reset()

	if len(art) > 8 {
		out[len(out)-2] = player.LatestMessage
	}

	for i := range art {
		if i < len(out) {
			fmt.Fprintf(&sbuilder, "   %s   %s", art[i], out[i])
			fmt.Println(sbuilder.String())
			sbuilder.Reset()
		} else {
			fmt.Fprintf(&sbuilder, "   %s", art[i])
			fmt.Println(sbuilder.String())
			sbuilder.Reset()
			if i == len(art)-1 {
				fmt.Fprintf(&sbuilder, "   %s", art[i])
				fmt.Print(sbuilder.String())
				sbuilder.Reset()
			}
		}
	}

}
