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

type Queue struct {
	streamers []beep.StreamSeeker
}

type Player struct {
	CurrentTrack  int
	LatestMessage string
	Status        string
	AlbumList     Album // not a list at the moment
	CurrentPos    string
	AlbumArt      image.Image
	Repeat        bool
	Random        bool
	RepeatOne     bool

	Timer  <-chan time.Time
	Format beep.Format
}

// response.Body doesn't implement Seek() method
// beep isn't bothered by this, but trying to
// call Seek() will fail since Len() will always return 0
// by using bytes.Reader and implementing empty Close() method
// we get io.ReadSeekCloser, which satisfies requirements of beep streamers
// (needs ReadCloser) and implements Seek() method
type Cached struct {
	*bytes.Reader
}

func (c Cached) Close() error {
	return nil
}

var player Player

func (q *Queue) Add(streamers ...beep.StreamSeeker) {
	q.streamers = append(q.streamers, streamers...)
}

func (q *Queue) ChangePosition(pos int) {
	if len(q.streamers) != 0 {
		newPos := q.streamers[0].Position()
		newPos += pos
		if newPos < 0 {
			newPos = 0
		}
		if newPos >= q.streamers[0].Len() {
			newPos = q.streamers[0].Len() - 1
		}
		if err := q.streamers[0].Seek(newPos); err != nil {
			log.Println(err)
		}
	}
}

func (q *Queue) Stream(samples [][2]float64) (n int, ok bool) {
	filled := 0
	for filled < len(samples) {
		if len(q.streamers) == 0 {
			player.Status = "Stopped:"
			for i := range samples[filled:] {
				samples[i][0] = 0
				samples[i][1] = 0
			}
			break
		} else {
			player.Status = "Playing:"
		}
		n, ok := q.streamers[0].Stream(samples[filled:])
		if !ok {
			// player.CurrentTrack = (player.CurrentTrack + 1) % player.AlbumList.Tracks.NumberOfItems
			// go q.FeedNewStream()
			player.printMetadata()
			q.streamers = q.streamers[1:]
		}
		filled += n
	}
	return len(samples), true
}

func (q *Queue) Err() error {
	return nil
}

// returns current position of current track
func (q *Queue) Position() int {
	if len(q.streamers) == 0 {
		return 0
	}
	return q.streamers[0].Position()
}

func (q *Queue) SkipForward() {
	q.streamers = q.streamers[1:]
	player.CurrentTrack = (player.CurrentTrack + 1) % player.AlbumList.Tracks.NumberOfItems
	go q.FeedNewStream()
}

func (q *Queue) SkipBackward() {
	q.streamers = q.streamers[1:]
	player.CurrentTrack = (player.AlbumList.Tracks.NumberOfItems + player.CurrentTrack - 1) % player.AlbumList.Tracks.NumberOfItems
	go q.FeedNewStream()
}

func (q *Queue) Next()

func (q *Queue) FeedNewStream() {
	item := player.AlbumList.Tracks.ItemListElement[player.CurrentTrack]
	for _, value := range item.TrackInfo.AdditionalProperty {
		// Hardcoded JSON field name
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
			player.LatestMessage = fmt.Sprint(player.AlbumList.ByArtist["name"], " - ", item.TrackInfo.Name, ".mp3 ",
				response.Status)

			body, _ := io.ReadAll(response.Body)
			buffer := &Cached{bytes.NewReader(body)}
			defer response.Body.Close()

			streamer, format, err := mp3.Decode(buffer)
			// TODO: not used, should do
			player.Format = format
			if err != nil {
				player.LatestMessage = fmt.Sprint(err)
			}

			speaker.Lock()
			q.Add(streamer)
			speaker.Unlock()
		}
	}
}

func main() {
	stdinReader := bufio.NewReader(os.Stdin)
	fmt.Print("Enter bandcamp link: ")
	input, err := stdinReader.ReadString('\n')
	if err != nil {
		log.Fatal(err)
	}
	input = strings.Trim(input, "\n")

	// TODO: link validation, note: not all bandcamp artist are hosted on whatever.bandcamp.com
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
		jsonstring, err = reader.ReadString(byte('\n'))
		if err != nil {
			log.Fatal(err)
		}
		if strings.Contains(jsonstring, "application/ld+json") {
			jsonstring, err = reader.ReadString(byte('\n'))
			if err != nil {
				log.Fatal(err)
			}
			break
		}
	}

	// TODO: track and album pages are different
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

	switch response.Header.Get("Content-Type") {
	case "image/jpeg":
		player.AlbumArt, err = jpeg.Decode(response.Body)
	case "image/png":
		player.AlbumArt, err = png.Decode(response.Body)
	}
	// TODO: add default case

	sr := beep.SampleRate(44100)
	speaker.Init(sr, sr.N(time.Second/10))
	var queue Queue

	ctrl := beep.Ctrl{Streamer: &queue, Paused: false}
	volume := &effects.Volume{Streamer: &ctrl, Base: 2}

	speaker.Play(volume)
	player.Status = "Playing:"
	// start playing first track in album
	go queue.FeedNewStream()
	player.Timer = time.Tick(time.Second)
	state := make(chan bool)

loop:
	for {
		select {
		case <-player.Timer:
			speaker.Lock()
			if player.Format.SampleRate == 0 {
				player.LatestMessage = "Error: division by zero"
			} else {
				player.CurrentPos = fmt.Sprint(player.Format.SampleRate.D(queue.Position()).Round(time.Second))
			}
			go player.printMetadata()
			speaker.Unlock()
		case <-state:
			player.LatestMessage = "test"
		}

		input, err = stdinReader.ReadString('\n')
		if err != nil {
			player.LatestMessage = err.Error()
		}

		switch strings.Trim(input, "\n") {
		case "m":
			speaker.Lock()
			volume.Silent = !volume.Silent
			speaker.Unlock()
		case "s":
			speaker.Lock()
			volume.Volume -= 0.5
			speaker.Unlock()
		case "w":
			speaker.Lock()
			volume.Volume += 0.5
			speaker.Unlock()
		case "a":
			speaker.Lock()
			queue.ChangePosition(0 - player.Format.SampleRate.N(time.Second*2))
			speaker.Unlock()
		case "d":
			speaker.Lock()
			queue.ChangePosition(player.Format.SampleRate.N(time.Second * 2))
			speaker.Unlock()
		case "p":
			speaker.Lock()
			player.Status = "Pause:"
			ctrl.Paused = !ctrl.Paused
			speaker.Unlock()
		case "f":
			speaker.Lock()
			queue.SkipForward()
			speaker.Unlock()
		case "b":
			speaker.Lock()
			queue.SkipBackward()
			speaker.Unlock()
		case "q":
			clearScreen()
			break loop
		}
	}
}

// clears screen, for now only unix will work, delete later in favor of more robust terminal drawing
func clearScreen() {
	cmd := exec.Command("clear")
	cmd.Stdout = os.Stdout
	cmd.Run()
}

func (player *Player) printMetadata() {
	clearScreen()

	item := player.AlbumList.Tracks.ItemListElement[player.CurrentTrack]
	//fmt.Printf("Lyrics:\n%s\n", item.TrackInfo.RecordingOf.Lyrics["text"])
	var seconds float64
	for _, value := range item.TrackInfo.AdditionalProperty {
		if value.Name == "duration_secs" {
			seconds = value.Value.(float64)
		}
	}
	out := strings.Split(convert.NewImageConverter().Image2ASCIIString(player.AlbumArt, &convert.DefaultOptions), "\n")

	var sbuilder strings.Builder
	fmt.Fprintf(&sbuilder, "%s   %s", out[2], player.Status)
	out[1] = sbuilder.String()
	sbuilder.Reset()

	fmt.Fprintf(&sbuilder, "%s   %d/%d %s", out[3], item.Position,
		player.AlbumList.Tracks.NumberOfItems, item.TrackInfo.Name)
	out[2] = sbuilder.String()
	sbuilder.Reset()

	fmt.Fprintf(&sbuilder, "%s   Artist: %s", out[4], player.AlbumList.ByArtist["name"])
	out[3] = sbuilder.String()
	sbuilder.Reset()

	fmt.Fprintf(&sbuilder, "%s   Album: %s", out[5], player.AlbumList.Name)
	out[4] = sbuilder.String()
	sbuilder.Reset()

	fmt.Fprintf(&sbuilder, "%s   Release Date: %s", out[6], player.AlbumList.DatePublished[:11])
	out[5] = sbuilder.String()
	sbuilder.Reset()

	fmt.Fprintf(&sbuilder, "%s   %s/%s", out[8], player.CurrentPos,
		time.Duration(seconds*float64(time.Second)).Round(time.Second))
	out[7] = sbuilder.String()
	sbuilder.Reset()

	fmt.Fprintf(&sbuilder, "%s   %s", out[len(out)-3], player.LatestMessage)
	out[len(out)-3] = sbuilder.String()
	sbuilder.Reset()

	for _, str := range out {
		fmt.Println(str)
	}
}
