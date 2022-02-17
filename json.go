package main

import (
	"errors"
	"net/url"
	"time"

	json "github.com/json-iterator/go"
)

var timeFormat = "02 Jan 2006 15:04:05 MST"
var timeZone = time.FixedZone("UTC", 0)

// FIXME: naming is a mess

// output types
type album struct {
	id                   int
	artId                int
	hasAudio             bool
	isBonus              bool
	isPreorder           bool
	albumIsPreorder      bool
	hasDiscounts         bool
	isPrivateStream      bool
	trAlbumSubscribeOnly bool
	itemType             string
	artist               string
	url                  string
	freeDownloadPage     string
	releaseType          string
	albumReleaseDate     time.Time
	dateErr              error
	bandId               int
	title                string
	modDate              time.Time
	tags                 []string
	totalTracks          int
}

type track struct {
	trackId         int
	streaming       int
	playCount       int
	isCapped        bool
	hasLyrics       bool
	unreleasedTrack bool
	hasFreeDownload bool
	albumPreorder   bool
	private         bool
	artist          string // for overrides
	url             string
	trackNumber     int
	title           string
	duration        float64
	lyrics          string
	mp3128          string
	mp3v0           string

	item *album
}

type trAlbum struct {
	ByArtist      Artist `json:"byArtist"`      // field "name" contains artist/band name
	Name          string `json:"name"`          // album/track name
	DatePublished string `json:"datePublished"` // release date
	// Image         string   `json:"image"`         // link to album art
	Tags        []string `json:"keywords"`    // tags/keywords
	Tracks      Track    `json:"track"`       // container for track data
	InAlbum     Album    `json:"inAlbum"`     // album name for tracks
	RecordingOf Lyrics   `json:"recordingOf"` // container for lyrics
}

type Artist struct {
	Name string `json:"name"`
}

type Track struct {
	NumberOfItems   int           `json:"numberOfItems"`   // total number of tracks
	ItemListElement []ListElement `json:"itemListElement"` // further container for track data
}

type ListElement struct {
	// Position  int  `json:"position"` // track number
	TrackInfo Item `json:"item"` // further container for track data
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
	AlbumReleaseType string `json:"albumReleaseType"` // not sure anymore
	NumTracks        int    `json:"numTracks"`        // same
	ByArtist         Artist `json:"byArtist"`         // field "name" contains artist/band name
}

// data-tralbum
type dataTrAlbum struct {
	Id                   int    `json:"id"`                      // item id
	ArtId                int    `json:"art_id"`                  // album cover ID
	HasAudio             bool   `json:"hasAudio"`                //
	IsBonus              bool   `json:"is_bonus"`                //
	IsPreorder           bool   `json:"is_preorder"`             //
	AlbumIsPreorder      bool   `json:"album_is_preorder"`       // TODO: test bools, might be usefull
	HasDiscounts         bool   `json:"has_discounts"`           //
	IsPrivateStream      bool   `json:"is_private_stream"`       //
	TrAlbumSubscribeOnly bool   `json:"tralbum_subscriber_only"` //
	ItemType             string `json:"item_type"`               //
	AlbumURL             string `json:"album_url"`               // doesn't exist on albums, null for singles
	URL                  string `json:"url"`                     // either album or track URL
	FreeDownloadPage     string `json:"freeDownloadPage"`        //
	Current              struct {
		AlbumId int    `json:"album_id"` // doesn't exist on albums
		BandId  int    `json:"band_id"`  //
		ModDate string `json:"mod_date"` //
	} `json:"current"`
	Trackinfo []struct {
		TrackId         int     `json:"track_id"`          //
		Streaming       int     `json:"streaming"`         //
		PlayCount       int     `json:"play_count"`        //
		TrackNum        int     `json:"track_num"`         //
		IsCapped        bool    `json:"is_capped"`         //
		HasLyrics       bool    `json:"has_lyrics"`        //
		UnreleasedTrack bool    `json:"unreleased_track"`  //
		HasFreeDownload bool    `json:"has_free_download"` //
		AlbumPreorder   bool    `json:"album_preorder"`    //
		Private         bool    `json:"private"`           //
		Artist          string  `json:"artist"`            // on albums with several artist might not be null
		TitleLink       string  `json:"title_link"`        //
		Duration        float64 `json:"duration"`          // duration in seconds
		File            struct {
			MP3128 string `json:"mp3-128"` // media url
			MP3V0  string `json:"mp3-v0"`  // available after login for bought items
		} `json:"file"` // file data
	} `json:"trackinfo"` // track list
}

// parses out data from ld+json and data-tralbum sections of media
// pages, hich combined contain all usefull metadata of media,
// expects valid escaped json strings as input
func parseTrAlbumJSON(ldJSON, tralbumJSON string) ([]track, error) {
	var metadata trAlbum
	var mediadata dataTrAlbum

	err := json.Unmarshal([]byte(ldJSON), &metadata)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal([]byte(tralbumJSON), &mediadata)
	if err != nil {
		return nil, err
	}

	metadataLen, mediadataLen := len(metadata.Tracks.ItemListElement), len(mediadata.Trackinfo)
	if metadataLen != 0 && metadataLen != mediadataLen || mediadataLen == 0 {
		return nil, errors.New("no media tracks were found")
	}

	// NOTE: in net.go page type is collected already,
	// but by not passing it to json.go we can let it
	// decide what to do with found data itself, so it
	// can be used without need to know type beforehand
	switch mediadata.ItemType {
	case "album":
		return extractAlbum(&metadata, &mediadata)
	case "track":
		return extractTrack(&metadata, &mediadata)
	default:
		return nil, errors.New("unexpected data")
	}
}

func extractAlbum(metadata *trAlbum, mediadata *dataTrAlbum) ([]track, error) {
	releaseDate, err := parseDate(metadata.DatePublished)
	// FIXME: if one wrong, second would also be wrong
	modDate, _ := parseDate(mediadata.Current.ModDate)
	albumMetadata := album{
		id:                   mediadata.Id,
		artId:                mediadata.ArtId,
		hasAudio:             mediadata.HasAudio,
		isBonus:              mediadata.IsBonus,
		isPreorder:           mediadata.IsPreorder,
		albumIsPreorder:      mediadata.AlbumIsPreorder,
		hasDiscounts:         mediadata.HasDiscounts,
		isPrivateStream:      mediadata.IsPrivateStream,
		trAlbumSubscribeOnly: mediadata.TrAlbumSubscribeOnly,
		itemType:             mediadata.ItemType,
		artist:               metadata.ByArtist.Name,
		url:                  mediadata.URL,
		freeDownloadPage:     mediadata.FreeDownloadPage,
		releaseType:          "",
		albumReleaseDate:     releaseDate,
		dateErr:              err,
		bandId:               mediadata.Current.BandId,
		title:                metadata.Name,
		modDate:              modDate,
		tags:                 metadata.Tags,
		totalTracks:          metadata.Tracks.NumberOfItems,
	}

	tracks := make([]track, len(mediadata.Trackinfo))
	u, err := url.Parse(albumMetadata.url)
	if err != nil {
		return nil, errors.New("json: album parser: failed to parse album url")
	}

	for i, item := range metadata.Tracks.ItemListElement {
		u.Path = mediadata.Trackinfo[i].TitleLink
		tracks[i] = track{
			trackId:         mediadata.Trackinfo[i].TrackId,
			streaming:       mediadata.Trackinfo[i].Streaming,
			playCount:       mediadata.Trackinfo[i].PlayCount,
			isCapped:        mediadata.Trackinfo[i].IsCapped,
			hasLyrics:       mediadata.Trackinfo[i].HasLyrics,
			unreleasedTrack: mediadata.Trackinfo[i].UnreleasedTrack,
			hasFreeDownload: mediadata.Trackinfo[i].HasFreeDownload,
			albumPreorder:   mediadata.Trackinfo[i].AlbumPreorder,
			private:         mediadata.Trackinfo[i].Private,
			artist:          mediadata.Trackinfo[i].Artist,
			url:             u.String(),
			trackNumber:     mediadata.Trackinfo[i].TrackNum,
			title:           item.TrackInfo.Name,
			duration:        mediadata.Trackinfo[i].Duration,
			lyrics:          item.TrackInfo.RecordingOf.Lyrics.Text,
			mp3128:          mediadata.Trackinfo[i].File.MP3128,
			mp3v0:           mediadata.Trackinfo[i].File.MP3V0,

			item: &albumMetadata,
		}
	}
	return tracks, nil
}

func extractTrack(metadata *trAlbum, mediadata *dataTrAlbum) ([]track, error) {
	releaseDate, err := parseDate(metadata.DatePublished)
	// FIXME: same for both track and album
	modDate, _ := parseDate(mediadata.Current.ModDate)

	// NOTE: field albumURL doesn't exist for albums and
	// single tracks without albums
	var albumURL string
	if mediadata.AlbumURL != "" {
		u, err := url.Parse(mediadata.URL)
		if err == nil {
			// NOTE: albumURL can be empty
			u.Path = mediadata.AlbumURL
			albumURL = u.String()
		}
	}

	// TODO: collect publishers name
	// NOTE: actual artist name can be in one of five places:
	// tralbum: .artist, .current.artist, .trackinfo[n].artist
	// ld+json: .byArtist.name, .inAlbum..byArtist.name, last
	// one usually has name that bandcamp displays on media
	// page, values for tracks in data-tralbum have artist on
	// items from various artists, first two are inconsistent,
	// can't see any logic behind value they actually hold
	var artist string
	if artist = metadata.InAlbum.ByArtist.Name; artist == "" {
		artist = metadata.ByArtist.Name
	}

	albumMetadata := &album{
		id:                   mediadata.Current.AlbumId,
		artId:                mediadata.ArtId,
		hasAudio:             mediadata.HasAudio,
		isBonus:              mediadata.IsBonus,
		isPreorder:           mediadata.IsPreorder,
		albumIsPreorder:      mediadata.AlbumIsPreorder,
		hasDiscounts:         mediadata.HasDiscounts,
		isPrivateStream:      mediadata.IsPrivateStream,
		trAlbumSubscribeOnly: mediadata.TrAlbumSubscribeOnly,
		itemType:             mediadata.ItemType,
		artist:               artist,
		url:                  albumURL,
		freeDownloadPage:     mediadata.FreeDownloadPage,
		releaseType:          metadata.InAlbum.AlbumReleaseType,
		albumReleaseDate:     releaseDate,
		dateErr:              err,
		bandId:               mediadata.Current.BandId,
		title:                metadata.InAlbum.Name,
		modDate:              modDate,
		tags:                 metadata.Tags,
		totalTracks:          -1, // still don't know how to extract total number from track page
	}

	tracks := make([]track, 1) // really hope that 'track' means 1 track always

	tracks[0] = track{
		trackId:         mediadata.Trackinfo[0].TrackId,
		streaming:       mediadata.Trackinfo[0].Streaming,
		playCount:       mediadata.Trackinfo[0].PlayCount,
		isCapped:        mediadata.Trackinfo[0].IsCapped,
		hasLyrics:       mediadata.Trackinfo[0].HasLyrics,
		unreleasedTrack: mediadata.Trackinfo[0].UnreleasedTrack,
		hasFreeDownload: mediadata.Trackinfo[0].HasFreeDownload,
		albumPreorder:   mediadata.Trackinfo[0].AlbumPreorder,
		private:         mediadata.Trackinfo[0].Private,
		artist:          mediadata.Trackinfo[0].Artist,
		url:             mediadata.URL,
		trackNumber:     mediadata.Trackinfo[0].TrackNum,
		title:           metadata.Name,
		duration:        mediadata.Trackinfo[0].Duration,
		lyrics:          metadata.RecordingOf.Lyrics.Text,
		mp3128:          mediadata.Trackinfo[0].File.MP3128,
		mp3v0:           mediadata.Trackinfo[0].File.MP3V0,

		item: albumMetadata,
	}
	return tracks, nil
}

func parseDate(input string) (time.Time, error) {
	date, err := time.ParseInLocation(timeFormat, input, timeZone)
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
	// TODO: remove first three
	page          int          `json:"-"`
	filters       string       `json:"-"`
	waiting       bool         `json:"-"`
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
		return nil, errors.New("tag page JSON parser: ./json.go:271: tab index out of range")
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
