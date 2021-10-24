package main

import (
	//"encoding/json"

	"errors"
	"fmt"
	"strings"
	"time"

	json "github.com/json-iterator/go"
	//"encoding/json"
)

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

type albumJSON struct {
	ByArtist      map[string]string `json:"byArtist"`      // field "name" contains artist/band name
	Name          string            `json:"name"`          // album title
	DatePublished string            `json:"datePublished"` // release date
	Image         string            `json:"image"`         // link to album art
	Tags          []string          `json:"keywords"`      // tags/keywords
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

type trackJSON struct {
	Name          string                 `json:"name"`
	Image         string                 `json:"image"`
	Tags          []string               `json:"keywords"`
	DatePublished string                 `json:"datePublished"`
	ByArtist      map[string]string      `json:"byArtist"`
	InAlbum       map[string]interface{} `json:"inAlbum"`
	RecordingOf   struct {
		Lyrics map[string]string `json:"lyrics"` // field "text" contains actual lyrics
	} `json:"recordingOf"` // container for lyrics
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
		artist:      metaData.ByArtist["name"],
		date:        metaData.DatePublished[:11], // TODO: do something with time in parsed date
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

func parseTrackJSON(metaDataJSON string, mediaDataJSON string) (albumMetaData *album) {
	var metaData trackJSON
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
		title:       metaData.InAlbum["name"].(string),
		artist:      metaData.ByArtist["name"],
		date:        metaData.DatePublished[:11], // TODO: do something with time in parsed date
		url:         mediaData.URL,
		tags:        fmt.Sprint(metaData.Tags),
		totalTracks: 1,
		tracks: []track{{
			trackNumber: 1,
			title:       metaData.Name,
			duration:    mediaData.Trackinfo[0].Duration,
			lyrics:      metaData.RecordingOf.Lyrics["text"],
			url:         mediaData.Trackinfo[0].File.MP3,
		}},
	}

	return albumMetaData
}

func getDummyData() *album {
	return &album{
		title:       "---",
		artist:      "---",
		date:        "---",
		url:         "https://golang.org",
		tags:        "[gopher music png]",
		totalTracks: 1,
		tracks: []track{{
			trackNumber: 1,
			title:       "---",
			duration:    0.0,
		}},
	}
}

func parseTagSearchHighlights(dataBlobJSON string) (urls []string) {
	var dataBlob tagSearchJSON
	err := json.Unmarshal([]byte(dataBlobJSON), &dataBlob)
	checkFatalError(err)
	if dataBlob.Hubs.IsSimple {
		return urls
	}
	for _, collection := range dataBlob.Hubs.Tabs[0].Collections {
		for _, item := range collection.Items {
			if value, ok := item["tralbum_url"].(string); ok {
				urls = append(urls, value)
			}
		}
	}
	return urls
}

type tagSearchJSON struct {
	Hubs Hub `json:"hub"`
}

type Hub struct {
	//RelatedTags []map[string]interface{} `json:"related_tags"`
	//Subgenres   []map[string]interface{} `json:"subgenres"`
	IsSimple bool  `json:"is_simple"`
	Tabs     []Tab `json:"tabs"`
}

type Tab struct {
	DigDeeper   Results      `json:"dig_deeper"`
	Collections []Collection `json:"collections"`
}

type Results struct {
	Result map[string]Collection `json:"results"`
}

type Collection struct {
	Items []map[string]interface{} `json:"items"`
}

func parseTagSearchJSON(dataBlobJSON string) (urls []string) {
	var dataBlob tagSearchJSON
	err := json.Unmarshal([]byte(dataBlobJSON), &dataBlob)
	checkFatalError(err)
	// FIXME: will absolutely fail at some point
	var index int
	if dataBlob.Hubs.IsSimple {
		index = 0
	} else {
		index = 1
	}

	if index > len(dataBlob.Hubs.Tabs)-1 {
		window.sendPlayerEvent(errors.New("tag page JSON parser: index out of range"))
		return urls
	}

	for _, collection := range dataBlob.Hubs.Tabs[index].DigDeeper.Result {
		for _, item := range collection.Items {
			if value, ok := item["tralbum_url"].(string); ok {
				urls = append(urls, value)
			}
		}
	}
	return urls
}
