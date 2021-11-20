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

// output types
type album struct {
	album       bool
	single      bool
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

// returns true and url if any streamable media was found
func (album *album) getURL(track int) (string, bool) {
	if album.tracks[track].url != "" {
		return album.tracks[track].url, true
	} else {
		return "", false
	}
}

// cache key = media url without any parameters
func (album *album) getTruncatedURL(track int) string {
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

type trAlbum struct {
	ByArtist      Artist   `json:"byArtist"`      // field "name" contains artist/band name
	Name          string   `json:"name"`          // album/track name
	DatePublished string   `json:"datePublished"` // release date
	Image         string   `json:"image"`         // link to album art
	Tags          []string `json:"keywords"`      // tags/keywords
	Tracks        Track    `json:"track"`         // container for track data
	InAlbum       Album    `json:"inAlbum"`       // album name
	RecordingOf   Lyrics   `json:"recordingOf"`   // same as in album json
}

type Artist struct {
	Name string `json:"name"`
}

type Track struct {
	NumberOfItems   int           `json:"numberOfItems"`   // total number of tracks
	ItemListElement []ListElement `json:"itemListElement"` // further container for track data
}

type ListElement struct {
	Position  int  `json:"position"` // track number
	TrackInfo Item `json:"item"`     // further container for track data
}

type Item struct {
	Name        string `json:"name"`        // track name
	RecordingOf Lyrics `json:"recordingOf"` // container for lyrics
}

type Lyrics struct {
	Lyrics Text `json:"lyrics"` // field "text" contains actual lyrics
}

type Text struct {
	Text string `json:"text"`
}

type Album struct {
	Name             string `json:"name"`             // album name for track
	AlbumReleaseType string `json:"albumReleaseType"` // only present in singles
	NumTracks        int    `json:"numTracks"`        // same
}

type media struct {
	//AlbumIsPreorder bool   `json:"album_is_preorder"` // unused, useless
	URL       string `json:"url"` // either album or track URL
	Trackinfo []struct {
		Duration float64 `json:"duration"` // duration in seconds
		File     struct {
			MP3 string `json:"mp3-128"` // media url
		} `json:"file"`
	} `json:"trackinfo"` // file data
}

func parseTrAlbumJSON(metadataJSON, mediaJSON string, isAlbum bool) (*album, error) {
	var metadata trAlbum
	var mediadata media
	err := json.Unmarshal([]byte(metadataJSON), &metadata)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal([]byte(mediaJSON), &mediadata)
	if err != nil {
		return nil, err
	}

	window.sendEvent(newDebugMessage(metadataJSON))
	if isAlbum {
		return extractAlbum(&metadata, &mediadata)
	}
	return extractTrack(&metadata, &mediadata)
}

func extractAlbum(metadata *trAlbum, mediadata *media) (*album, error) {
	albumMetadata := &album{
		album:       true,
		single:      false,
		imageSrc:    metadata.Image,
		title:       metadata.Name,
		artist:      metadata.ByArtist.Name,
		date:        parseDate(metadata.DatePublished),
		url:         mediadata.URL,
		tags:        strings.Join(metadata.Tags, " "),
		totalTracks: metadata.Tracks.NumberOfItems,
	}

	if len(metadata.Tracks.ItemListElement) == len(mediadata.Trackinfo) {
		for i, item := range metadata.Tracks.ItemListElement {
			albumMetadata.tracks = append(albumMetadata.tracks,
				track{
					trackNumber: item.Position,
					title:       item.TrackInfo.Name,
					duration:    mediadata.Trackinfo[i].Duration,
					lyrics:      item.TrackInfo.RecordingOf.Lyrics.Text,
					url:         mediadata.Trackinfo[i].File.MP3,
				})
		}
	} else {
		return nil, errors.New("not enough data was parsed")
	}
	return albumMetadata, nil
}

func extractTrack(metadata *trAlbum, mediadata *media) (*album, error) {
	albumMetadata := &album{
		album:       false,
		imageSrc:    metadata.Image,
		title:       metadata.InAlbum.Name,
		artist:      metadata.ByArtist.Name,
		date:        parseDate(metadata.DatePublished),
		url:         mediadata.URL,
		tags:        strings.Join(metadata.Tags, " "),
		totalTracks: 1,
	}

	if metadata.InAlbum.AlbumReleaseType == "SingleRelease" {
		albumMetadata.single = true
	}

	if len(mediadata.Trackinfo) > 0 {
		albumMetadata.tracks = append(albumMetadata.tracks,
			track{
				trackNumber: 1,
				title:       metadata.Name,
				duration:    mediadata.Trackinfo[0].Duration,
				lyrics:      metadata.RecordingOf.Lyrics.Text,
				url:         mediadata.Trackinfo[0].File.MP3,
			},
		)
	} else {
		return nil, errors.New("not enough data was parsed")
	}

	return albumMetadata, nil
}

func parseDate(input string) (strDate string) {
	date, err := time.Parse("02 Jan 2006 15:04:05 GMT", input)
	if err != nil {
		window.sendEvent(newErrorMessage(err))
		strDate = "---"
	} else {
		y, m, d := date.Date()
		strDate = fmt.Sprintf("%02d %s %4d", d, strings.ToLower(m.String()[:3]), y)
	}
	return strDate
}

// tag search results
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
	Collections []Result `json:"collections"`
	DigDeeper   Results  `json:"dig_deeper"`
}

// NOTE: key for accessing underlying data is dynamic
// ({\"tags\":[\"tag\"],\"format\":\"all\",\"location\":0,\"sort\":\"pop\"})
// key itself is stored in dig_deeper.initial_settings
type Results struct {
	InitialSettings string            `json:"initial_settings"`
	Result          map[string]Result `json:"results"`
}

type Result struct {
	MoreAvailable bool         `json:"more_available"`
	Items         []SearchItem `json:"items"`
}

type SearchItem struct {
	Title  string `json:"title"`       // title
	Artist string `json:"artist"`      // artist
	Genre  string `json:"genre"`       // genre
	URL    string `json:"tralbum_url"` // tralbum_url
	ArtId  int    `json:"art_id"`      // art_id
}

func parseTagSearchJSON(dataBlobJSON string, highlights bool) (*Result, error) {
	var dataBlob tagSearchJSON
	var searchResults Result
	err := json.Unmarshal([]byte(dataBlobJSON), &dataBlob)
	if err != nil {
		return &searchResults, err
	}

	if highlights {
		// first tab is highlights, second one has actual search results
		// highlights tab has several sections with albums/tracks
		// for highlights query go through all sections and collect all data
		// we shouldn't be here if tag is simple, NOTE: that some sections
		// have empty positions, haven't figured out what they actually are
		if dataBlob.Hubs.IsSimple || len(dataBlob.Hubs.Tabs) == 0 {
			return &searchResults, errors.New("nothing was found")
		}
		for _, collection := range dataBlob.Hubs.Tabs[0].Collections {
			searchResults.Items = append(searchResults.Items, collection.Items...)
		}
		return &searchResults, nil
	}

	// FIXME: will absolutely fail at some point
	var index int
	// simple = tag is not "genre" and doesn't have tabs on tag search page
	// for not simple tags there are two tabs: highlights and all releases
	if !dataBlob.Hubs.IsSimple {
		index = 1
	}

	if index > len(dataBlob.Hubs.Tabs)-1 {
		window.sendEvent(newDebugMessage(fmt.Sprint(dataBlob.Hubs.Tabs)))
		return &searchResults, errors.New("tag page JSON parser: index out of range")
	}

	key := dataBlob.Hubs.Tabs[index].DigDeeper.InitialSettings
	if value, ok := dataBlob.Hubs.Tabs[index].DigDeeper.Result[key]; ok {
		return &value, nil
	}
	return &searchResults, errors.New("nothing was found")
}

/*func extractResults(results []byte) (*Result, error) {

	var result Result
	if !json.Valid(results) {
		window.sendEvent(newDebugMessage(string(results)))
		return &result, errors.New("extractResults: got invalid JSON")
	}

	err := json.Unmarshal(results, &result)
	if err != nil {
		return &result, err
	}

	return &result, nil
} */
