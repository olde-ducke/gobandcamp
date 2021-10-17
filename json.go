package main

import (
	//"encoding/json"

	"fmt"
	"strings"
	"time"

	json "github.com/json-iterator/go"
)

// TODO: maybe it would be a good idea to wrap everything in one more
// sensible struct and return it instead of this nonsense

type album struct {
	imageSrc    string
	title       string
	artist      string
	date        string
	url         string
	tags        string
	totalTracks int
	tracks      []track
}

type track struct {
	trackNumber int
	title       string
	duration    float64
	lyrics      string
	url         string
}

type albumJSON struct {
	ByArtist      map[string]interface{} `json:"byArtist"`      // field "name" contains artist/band name
	Name          string                 `json:"name"`          // album title
	DatePublished string                 `json:"datePublished"` // release date
	Image         string                 `json:"image"`         // link to album art
	Tags          []string               `json:"keywords"`      // tags/keywords
	Tracks        struct {
		NumberOfItems   int `json:"numberOfItems"` // total number of tracks
		ItemListElement []struct {
			Position  int `json:"position"` // track number
			TrackInfo struct {
				Name string `json:"name"` // track name
				//Duration           string     `json:"duration"`           // string representation of duration P##H##M##S:
				RecordingOf struct {
					Lyrics map[string]string `json:"lyrics"` // field "text" contains actual lyrics
				} `json:"recordingOf"` // container for lyrics
				/*AdditionalProperty []struct {
					Name  string      `json:"name"`
					Value interface{} `json:"value"`
				} `json:"additionalProperty"` // list of containers for additional info*/
			} `json:"item"` // further container for track data
		} `json:"itemListElement"` // further container for track data
	} `json:"track"` // container for track data
}

type mediaJSON struct {
	AlbumIsPreorder bool   `json:"album_is_preorder"`
	URL             string `json:"url"`
	Trackinfo       []struct {
		Duration float64 `json:"duration"`
		File     struct {
			MP3 string `json:"mp3-128"`
		} `json:"file"`
	} `json:"trackinfo"`
}

// TODO: track and album pages are different,
// only album pages are implemented
func parseAlbumJSON(metaDataJSON string, mediaDataJSON string) (albumMetaData *album) {
	var metaData albumJSON
	var mediaData mediaJSON
	err := json.Unmarshal([]byte(metaDataJSON), &metaData)
	// TODO: don't crash?
	if err != nil {
		checkFatalError(err)
	}
	err = json.Unmarshal([]byte(mediaDataJSON), &mediaData)
	if err != nil {
		checkFatalError(err)
	}
	albumMetaData = &album{
		imageSrc:    metaData.Image,
		title:       metaData.Name,
		artist:      metaData.ByArtist["name"].(string),
		date:        metaData.DatePublished[:11],
		url:         mediaData.URL,
		tags:        fmt.Sprint(metaData.Tags),
		totalTracks: metaData.Tracks.NumberOfItems,
	}

	for i, item := range metaData.Tracks.ItemListElement {
		albumMetaData.tracks = append(albumMetaData.tracks,
			track{
				trackNumber: item.Position,
				title:       item.TrackInfo.Name,
				duration:    mediaData.Trackinfo[i].Duration,
				lyrics:      item.TrackInfo.RecordingOf.Lyrics["text"],
				url:         mediaData.Trackinfo[i].File.MP3,
			})

	}
	return albumMetaData
}

func getDummyData() *album {
	return &album{
		title:       "---",
		artist:      "---",
		date:        "01 jan 1900",
		url:         "https://golang.org",
		tags:        "[gopher music png]",
		totalTracks: 3,
		tracks: []track{{
			trackNumber: 2,
			title:       "---",
			duration:    200.5,
		}},
	}
}

func (metadata *album) formatString(n int) string {
	sbuilder := strings.Builder{}
	fmt.Fprintf(&sbuilder, "%s\n by %s\nreleased %s\n%s\n\n%s %2d/%d - %s\n%s\n%s/%s\nvolume %s mode %s\n\n\n\n\n%s",
		metadata.title,
		metadata.artist,
		metadata.date,
		metadata.tags,
		`%2s`,
		n+1,
		metadata.totalTracks,
		metadata.tracks[n].title,
		`%s`, `%s`,
		(time.Duration(metadata.tracks[n].duration) * time.Second).Round(time.Second),
		`%4s`, `%s`,
		metadata.url,
	)
	defer sbuilder.Reset()
	return sbuilder.String()
}
