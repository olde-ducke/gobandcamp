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

// TODO: move these methods away from json, they have nothing to do with it
func (album *album) formatString(n int) string {
	sbuilder := strings.Builder{}
	fmt.Fprintf(&sbuilder, "%s\n by %s\nreleased %s\n%s\n\n%s %2d/%d - %s\n%s\n%s/%s\nvolume %s mode %s\n\n\n\n\n%s",
		album.title,
		album.artist,
		album.date,
		album.tags,
		`%2s`,
		n+1,
		album.totalTracks,
		album.tracks[n].title,
		`%s`, `%s`,
		(time.Duration(album.tracks[n].duration) * time.Second).Round(time.Second),
		`%4s`, `%s`,
		album.url,
	)
	defer sbuilder.Reset()
	return sbuilder.String()
}

// returns true and url if any streamable media was found
func (album *album) getURL(track int) (string, bool) {
	if album.tracks[track].url != "" {
		return album.tracks[track].url, true
	} else {
		return "", false
	}
}

// cache key = media url without any parameters
func (album *album) getCacheID(track int) string {
	return getTruncatedURL(album.tracks[track].url)
}

// a<album_art_id>_nn.jpg
// other images stored without type prefix?
// not all sizes are listed here, all up to _16 are existing files
// _10 - original, whatever size it was
// _16 - 700x700
// _7  - 160x160
// _3  - 100x100
func (album *album) getImageURL(size int) string {
	var s string
	switch size {
	case 3:
		s = "_16"
	case 2:
		s = "_7"
	case 1:
		s = "_3"
	default:
		return album.imageSrc
	}
	return strings.Replace(album.imageSrc, "_10", s, 1)
}

// TODO: unwrap all types back, it's painfull to look at this thing below
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
				Name        string `json:"name"` // track name
				RecordingOf struct {
					Lyrics map[string]string `json:"lyrics"` // field "text" contains actual lyrics
				} `json:"recordingOf"` // container for lyrics
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

func parseAlbumJSON(metaDataJSON string, mediaDataJSON string) (*album, error) {
	var metaData albumJSON
	var mediaData mediaJSON
	err := json.Unmarshal([]byte(metaDataJSON), &metaData)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal([]byte(mediaDataJSON), &mediaData)
	if err != nil {
		return nil, err
	}

	albumMetaData := &album{
		imageSrc:    metaData.Image,
		title:       metaData.Name,
		artist:      metaData.ByArtist["name"],
		date:        parseDate(metaData.DatePublished),
		url:         mediaData.URL,
		tags:        strings.Join(metaData.Tags, " "),
		totalTracks: metaData.Tracks.NumberOfItems,
	}

	if len(metaData.Tracks.ItemListElement) == len(mediaData.Trackinfo) {
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
	} else {
		return nil, errors.New("not enough data was parsed")
	}
	return albumMetaData, nil
}

func parseTrackJSON(metaDataJSON string, mediaDataJSON string) (*album, error) {
	var metaData trackJSON
	var mediaData mediaJSON
	err := json.Unmarshal([]byte(metaDataJSON), &metaData)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal([]byte(mediaDataJSON), &mediaData)
	if err != nil {
		return nil, err
	}

	albumMetaData := &album{
		imageSrc:    metaData.Image,
		title:       metaData.InAlbum["name"].(string),
		artist:      metaData.ByArtist["name"],
		date:        parseDate(metaData.DatePublished),
		url:         mediaData.URL,
		tags:        strings.Join(metaData.Tags, " "),
		totalTracks: 1,
	}

	if len(mediaData.Trackinfo) > 0 {
		albumMetaData.tracks = append(albumMetaData.tracks,
			track{
				trackNumber: 1,
				title:       metaData.Name,
				duration:    mediaData.Trackinfo[0].Duration,
				lyrics:      metaData.RecordingOf.Lyrics["text"],
				url:         mediaData.Trackinfo[0].File.MP3,
			},
		)
	} else {
		return nil, errors.New("not enough data was parsed")
	}

	return albumMetaData, nil
}

func parseDate(input string) (strDate string) {
	date, err := time.Parse("02 Jan 2006 00:00:00 GMT", input)
	if err != nil {
		window.sendEvent(newErrorMessage(err))
		strDate = "---"
	} else {
		y, m, d := date.Date()
		strDate = fmt.Sprintf("%02d %s %4d", d, strings.ToLower(m.String()[:3]), y)
	}
	return strDate
}

func getDummyData() *album {
	return &album{
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
}

func parseTagSearchHighlights(dataBlobJSON string) (urls []string) {
	var dataBlob tagSearchJSON
	err := json.Unmarshal([]byte(dataBlobJSON), &dataBlob)
	checkFatalError(err)
	if dataBlob.Hubs.IsSimple || len(dataBlob.Hubs.Tabs) == 0 {
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
		window.sendEvent(newErrorMessage(errors.New("tag page JSON parser: index out of range")))
		return urls
	}

	for _, collection := range dataBlob.Hubs.Tabs[index].DigDeeper.Result {
		for _, item := range collection.Items {
			if value, ok := item["tralbum_url"].(string); ok {
				urls = append(urls, value)
			} else {
				return urls
			}
		}
	}
	return urls
}
