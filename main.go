package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/faiface/beep"
	"github.com/faiface/beep/mp3"
	"github.com/faiface/beep/speaker"
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
	streamers []beep.Streamer
}

type Player struct {
	CurrentTrack  int
	LatestMessage string
	Status        string
	AlbumList     Album // not a list at the moment
}

var player Player

func (q *Queue) Add(streamers ...beep.Streamer) {
	q.streamers = append(q.streamers, streamers...)
}

func (q *Queue) Stream(samples [][2]float64) (n int, ok bool) {
	filled := 0
	if player.CurrentTrack == 0 {
		player.printMetadata(player.CurrentTrack)
		player.CurrentTrack++
	}
	for filled < len(samples) {
		if len(q.streamers) == 0 {
			for i := range samples[filled:] {
				samples[i][0] = 0
				samples[i][1] = 0
			}
			break
		}
		n, ok := q.streamers[0].Stream(samples[filled:])
		if !ok {
			q.streamers = q.streamers[1:]
			player.printMetadata(player.CurrentTrack)
			player.CurrentTrack++
		}
		filled += n
	}
	return len(samples), true
}

func (q *Queue) Err() error {
	return nil
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
	// set mobile view, weights a bit less than desktop version
	request.Header.Set("Cookie", "mvp=p")

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

	// var album Album
	// TODO: track and album pages are different
	err = json.Unmarshal([]byte(jsonstring), &player.AlbumList)
	if err != nil {
		log.Fatal(err)
	}

	sr := beep.SampleRate(44100)
	speaker.Init(sr, sr.N(time.Second/10))
	var queue Queue
	speaker.Play(&queue)

	for _, item := range player.AlbumList.Tracks.ItemListElement {
		for _, value := range item.TrackInfo.AdditionalProperty {
			if value.Name == "file_mp3-128" {

				// TODO: not all tracks are streamable on service, pretty sure there was "streamable" field in JSON
				// new request to media server
				request, err = http.NewRequest("GET", value.Value.(string), nil)
				if err != nil {
					log.Fatal(err)
				}
				// lets pretend that we are chrome on NT
				request.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/93.0.4577.82 Safari/537.36")
				response, err = http.DefaultClient.Do(request)
				if err != nil {
					log.Fatal(err)
				}
				if response.StatusCode > 299 {
					log.Fatalf("Request failed with status code: %d\n", response.StatusCode)
				}
				player.LatestMessage = fmt.Sprint(player.AlbumList.ByArtist["name"], " - ", item.TrackInfo.Name, ".mp3 ",
					response.Status)

				streamer, format, err := mp3.Decode(response.Body)
				if err != nil {
					player.LatestMessage = fmt.Sprint(err)
					continue
				}

				resampled := beep.Resample(4, format.SampleRate, sr, streamer)
				speaker.Lock()
				queue.Add(resampled)
				speaker.Unlock()
			}
		}
	}
	select {}
}

// clears screen, for now only unix will work
func clearScreen() {
	cmd := exec.Command("clear") //Linux example, its tested
	cmd.Stdout = os.Stdout
	cmd.Run()
}

func (player *Player) printMetadata(n int) {
	clearScreen()
	if player.AlbumList.Tracks.NumberOfItems == n {
		player.LatestMessage = "last track finished playing"
		player.Status = "Stopped:"
		player.CurrentTrack = player.AlbumList.Tracks.NumberOfItems - 1
	} else {
		player.Status = "Playing:"
	}
	item := player.AlbumList.Tracks.ItemListElement[player.CurrentTrack]
	fmt.Println(player.Status)
	fmt.Printf("%d/%d %s %s \n", item.Position, player.AlbumList.Tracks.NumberOfItems, item.TrackInfo.Duration, item.TrackInfo.Name)
	fmt.Printf("Artist: %s\nAlbum: %s\nRelease Date: %s\nAlbum art: %s\n", player.AlbumList.ByArtist["name"], player.AlbumList.Name, player.AlbumList.DatePublished, player.AlbumList.Image)
	fmt.Printf("Lyrics:\n%s\n", item.TrackInfo.RecordingOf.Lyrics["text"])
	for _, value := range item.TrackInfo.AdditionalProperty {
		if value.Name == "duration_secs" {
			fmt.Println("Duration in seconds: ", value.Value)
		}
		if value.Name == "file_mp3-128" {
			fmt.Println("Media Link: ", value.Value.(string))
		}
	}
	fmt.Println(player.LatestMessage)
}
