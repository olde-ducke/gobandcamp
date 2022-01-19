package main

import (
	//"encoding/json"

	"errors"
	"time"

	json "github.com/json-iterator/go"
	//"encoding/json"
)

// output types
type album struct {
	// imageSrc    string
	album       bool
	single      bool
	artID       int
	title       string
	artist      string
	date        time.Time
	url         string
	tags        []string
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

type trAlbum struct {
	ByArtist      Artist `json:"byArtist"`      // field "name" contains artist/band name
	Name          string `json:"name"`          // album/track name
	DatePublished string `json:"datePublished"` // release date
	// Image         string   `json:"image"`         // link to album art
	Tags        []string `json:"keywords"`    // tags/keywords
	Tracks      Track    `json:"track"`       // container for track data
	InAlbum     Album    `json:"inAlbum"`     // album name
	RecordingOf Lyrics   `json:"recordingOf"` // same as in album json
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
	ArtId     int    `json:"art_id"`
	URL       string `json:"url"` // either album or track URL
	Trackinfo []struct {
		Duration float64 `json:"duration"` // duration in seconds
		File     struct {
			MP3128 string `json:"mp3-128"` // media url
			// higher quality mp3-v0 available only after login
			// and only for some items
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

	if isAlbum {
		return extractAlbum(&metadata, &mediadata)
	}
	return extractTrack(&metadata, &mediadata)
}

func extractAlbum(metadata *trAlbum, mediadata *media) (*album, error) {
	date, err := parseDate(metadata.DatePublished)
	if err != nil {
		return nil, err
	}
	albumMetadata := &album{
		// imageSrc:    metadata.Image,
		album:       true,
		single:      false,
		artID:       mediadata.ArtId,
		title:       metadata.Name,
		artist:      metadata.ByArtist.Name,
		date:        date,
		url:         mediadata.URL,
		tags:        metadata.Tags,
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
					url:         mediadata.Trackinfo[i].File.MP3128,
				})
		}
	} else {
		return nil, errors.New("not enough data was parsed")
	}
	return albumMetadata, nil
}

func extractTrack(metadata *trAlbum, mediadata *media) (*album, error) {
	date, err := parseDate(metadata.DatePublished)
	if err != nil {
		return nil, err
	}
	albumMetadata := &album{
		// imageSrc:    metadata.Image,
		album:       false,
		artID:       mediadata.ArtId,
		title:       metadata.InAlbum.Name,
		artist:      metadata.ByArtist.Name,
		date:        date,
		url:         mediadata.URL,
		tags:        metadata.Tags,
		totalTracks: 1,
	}

	// FIXME: would crash if InAlbum is nil, same with others
	// needs more thorough check
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
				url:         mediadata.Trackinfo[0].File.MP3128,
			},
		)
	} else {
		return nil, errors.New("not enough data was parsed")
	}

	return albumMetadata, err
}

func parseDate(input string) (time.Time, error) {
	date, err := time.Parse("02 Jan 2006 15:04:05 MST", input)
	if err != nil {
		return date, err
	}
	return date, nil
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

// TODO: collections have types, some contain fan reviews for albums
// at least 1 collection doesn't have media items
// filter them by type
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
	page          int
	filters       string
	waiting       bool
	MoreAvailable bool         `json:"more_available"`
	Items         []SearchItem `json:"items"`
}

type SearchItem struct {
	Type   string `json:"tralbum_type"`
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
		// we shouldn't be here if tag is simple
		if dataBlob.Hubs.IsSimple || len(dataBlob.Hubs.Tabs) == 0 {
			return nil, errors.New("nothing was found")
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
		return nil, errors.New("tag page JSON parser: ./json.go:265: tab index out of range")
	}

	key := dataBlob.Hubs.Tabs[index].DigDeeper.InitialSettings
	if value, ok := dataBlob.Hubs.Tabs[index].DigDeeper.Result[key]; ok {
		value.page = 2
		value.filters = dataBlob.Hubs.Tabs[index].DigDeeper.InitialSettings
		return &value, nil
	}

	return nil, errors.New("nothing was found")
}

func extractResults(results []byte) (*Result, error) {

	var result Result

	err := json.Unmarshal(results, &result)
	if err != nil {
		return nil, err
	}

	return &result, nil
}
