package main

import (
	//"encoding/json"
	"image"

	json "github.com/json-iterator/go"
)

// TODO: struct names and fields are complete mess
// TODO: maybe it would be a good idea to wrap everything in one more
// sensible struct and return it instead of this nonsense
type Album struct {
	ByArtist      map[string]interface{} `json:"byArtist"`      // field "name" contains artist/band name
	Name          string                 `json:"name"`          // album title
	DatePublished string                 `json:"datePublished"` // release date
	Image         string                 `json:"image"`         // link to album art
	Tracks        Track                  `json:"track"`         // container for track data
	Tags          []string               `json:"keywords"`      // tags/keywords
	// TODO: move image somewhere else
	AlbumArt image.Image
}

type Track struct {
	NumberOfItems   int        `json:"numberOfItems"`   // total number of tracks
	ItemListElement []ItemList `json:"itemListElement"` // further container for track data
}

type ItemList struct {
	Position  int  `json:"position"` // track number
	TrackInfo Item `json:"item"`     // further container for track data
}

type Item struct {
	Name               string     `json:"name"`               // track name
	RecordingOf        Lyric      `json:"recordingOf"`        // container for lyrics
	Duration           string     `json:"duration"`           // string representation of duration P##H##M##S:
	AdditionalProperty []Property `json:"additionalProperty"` // list of containers for additional info (link to media, duration in float, and useless info)
}

type Lyric struct {
	Lyrics map[string]string `json:"lyrics"` // field "text" contains actual lyrics
}

type Property struct {
	Name  string      `json:"name"`
	Value interface{} `json:"value"`
}

func parseJSON(jsonstring string) *Album {
	var album Album
	// TODO: track and album pages are different,
	// only album pages are implemented
	err := json.Unmarshal([]byte(jsonstring), &album)
	if err != nil {
		checkFatalError(err)
	}
	return &album
}
