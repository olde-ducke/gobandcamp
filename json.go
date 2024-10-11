package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// output types
type album struct {
	// imageSrc    string
	album       bool
	single      bool
	artID       uint64
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
	ArtId     uint64 `json:"art_id"`
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
	albumMetadata := &album{
		// imageSrc:    metadata.Image,
		album:       true,
		single:      false,
		artID:       mediadata.ArtId,
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
					url:         mediadata.Trackinfo[i].File.MP3128,
				})
		}
	} else {
		return nil, errors.New("not enough data was parsed")
	}
	return albumMetadata, nil
}

func extractTrack(metadata *trAlbum, mediadata *media) (*album, error) {
	albumMetadata := &album{
		// imageSrc:    metadata.Image,
		album:       false,
		artID:       mediadata.ArtId,
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
				url:         mediadata.Trackinfo[0].File.MP3128,
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
		strDate = fmt.Sprintf("%s %d, %4d", m, d, y)
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

type Format uint8

const (
	All Format = iota
	Digital
	Vinyl
	CompactDisks
	Cassettes
	TShirts
)

func FormatFromString(input string) (Format, error) {
	switch input {
	case "all":
		return All, nil
	case "digital":
		return Digital, nil
	case "vinyl":
		return Vinyl, nil
	case "cd":
		return CompactDisks, nil
	case "cassette":
		return Cassettes, nil
	case "t-shirt":
		return TShirts, nil
	default:
		return All, errors.New("unknown format: \"" + input + "\"")
	}
}

type DiscoverRequest struct {
	CategoryID         Format   `json:"category_id"`
	Cursor             string   `json:"cursor"`
	GeonameID          int64    `json:"geoname_id"`
	IncludeResultTypes []string `json:"include_result_types"`
	Size               int64    `json:"size"`
	Slice              string   `json:"slice"`
	TagNormNames       []string `json:"tag_norm_names"`
}

func (req DiscoverRequest) String() string {
	var sb strings.Builder
	defer sb.Reset()

	enc := json.NewEncoder(&sb)
	enc.SetEscapeHTML(false)

	if err := enc.Encode(&req); err != nil {
		return err.Error()
	}

	return sb.String()
}

type DiscoverResult struct {
	Request          *DiscoverRequest `json:"-"`
	Results          []Result         `json:"results"`
	ResultCount      uint64           `json:"result_count"`
	BatchResultCount uint64           `json:"batch_result_count"`
	Cursor           *string          `json:"cursor"`
	DiscoverSpecID   int64            `json:"discover_spec_id"`
	APISpecial       string           `json:"__api_special__,omitempty"`
	ErrorType        string           `json:"error_type,omitempty"`
}

// TODO: added only bare minimum
type Result struct {
	ItemURL      string  `json:"item_url"`
	ResultType   string  `json:"result_type"`
	ItemImageID  uint64  `json:"item_image_id"`
	AlbumArtist  *string `json:"album_artist"`
	BandName     string  `json:"band_name"`
	ReleaseDate  string  `json:"release_date"`
	ItemDuration float64 `json:"item_duration"`
	Title        string  `json:"title"`
	TrackCount   uint64  `json:"track_count"`
}

/*
func formatResult(result map[string]any) {
	var by any
	if result["album_artist"] != nil {
		by = result["album_artist"]
	} else {
		by = result["band_name"]
	}

	t, err := time.Parse("2006-01-02 15:04:05 MST", result["release_date"].(string))
	if err != nil {
		e.Println(err)
		return
	}

	d := math.Round(result["item_duration"].(float64) / 60.0)

	l.Printf("\x1b[1m%v\x1b[0m\nby \x1b[1m%v\x1b[0m\n%v tracks, %.0f minutes\nreleased %s\n%v",
		result["title"],
		by,
		result["track_count"],
		d,
		t.Format("January 02, 2006"),
		result["item_url"],
	)
}
*/

func (res DiscoverResult) String() string {
	var sb strings.Builder
	defer sb.Reset()

	enc := json.NewEncoder(&sb)
	enc.SetEscapeHTML(false)

	if err := enc.Encode(&res); err != nil {
		return err.Error()
	}

	return sb.String()
}
