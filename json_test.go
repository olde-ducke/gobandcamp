package main

import (
	"bufio"
	"log"
	"os"
	"testing"
	"time"
)

var metaData, mediaData string
var wantData = &album{
	album:       true,
	single:      false,
	artID:       255644,
	title:       "album_name_test",
	artist:      "artist_name_test",
	date:        time.Date(1970, time.January, 1, 0, 0, 0, 0, time.FixedZone("GMT", 0)),
	url:         "https://gopher.example.com/album/album_name_test",
	tags:        []string{"gopher", "music", "png"},
	totalTracks: 3,
	tracks: []track{
		{
			trackNumber: 1,
			title:       "testing",
			duration:    202.241,
			lyrics:      "testing\r\nlyrics\r\non\r\nfirst\r\ntrack",
			url:         "https://prefix.example.com/stream/uuid/mp3-128/7646382?p=0&amp;ts=timestamp&amp;t=another_uuid&amp;token=timestamp_token",
		},
		{
			trackNumber: 2,
			title:       "track",
			duration:    202.897,
			lyrics:      "testing\r\nlyrics\r\non\r\nsecond\r\ntrack",
			url:         "",
		},
		{
			trackNumber: 3,
			title:       "titles",
			duration:    836.75,
			lyrics:      "",
			url:         "https://prefix.example.com/stream/uuid/mp3-128/12354221?p=0&amp;ts=timestamp&amp;t=another_uuid&amp;token=timestamp_token",
		},
	},
}
var formatStr = "%s\nwant: %v\n got: %v"
var formatStrLong = "%s\nwant: %s %v %s %v\n got: %s %v %s %v"

func TestParseTrAlbumJSONError(t *testing.T) {
	// empty second string
	gotData, err := parseTrAlbumJSON(metaData, "", true)
	if gotData != nil || err == nil {
		t.Errorf(formatStrLong, "should return error and no value",
			"\n", nil, "\n", "<error value>", "\n", gotData, "\n", err)
	}
	// empty first string
	gotData, err = parseTrAlbumJSON("", mediaData, true)
	if gotData != nil || err == nil {
		t.Errorf(formatStrLong, "should return error and no value",
			"\n", nil, "\n", "<error value>", "\n", gotData, "\n", err)
	}
	// wrong json strings
	gotData, err = parseTrAlbumJSON(mediaData, metaData, false)
	if gotData != nil || err == nil {
		t.Errorf(formatStrLong, "should return error and no value",
			"", nil, "", "<error value>", "", gotData, "", err)
	}
}

func TestParseAlbumJSONFake(t *testing.T) {
	gotData, err := parseTrAlbumJSON(metaData, mediaData, true)
	// check if got anything at all
	if gotData == nil || err != nil {
		t.Fatalf(formatStrLong, "value should not be null, no error is expected",
			"", wantData, "", nil, "", gotData, "", err)
	}

	// check that number of tracks is equal to expected value
	if len(gotData.tracks) != gotData.totalTracks ||
		len(gotData.tracks) != len(wantData.tracks) ||
		gotData.totalTracks != wantData.totalTracks {
		t.Fatalf(formatStrLong, "wrong item count",
			"len(data.tracks) ==", len(wantData.tracks),
			"data.totalTracks ==", wantData.totalTracks,
			"len(data.tracks) ==", len(gotData.tracks),
			"data.totalTracks ==", gotData.totalTracks)
	}

	// check item type
	if gotData.album != wantData.album || gotData.single != wantData.single {
		t.Errorf(formatStrLong, "wrong item type",
			"album:", wantData.album, "single:", wantData.single,
			"album:", gotData.album, "single:", gotData.single)
	}

	// check album cover url
	// TODO: change album art to art_id ???
	if gotData.artID != wantData.artID {
		t.Errorf(formatStr, "wrong album art utl", wantData.artID, gotData.artID)
	}

	// check album metadata
	if gotData.title != wantData.title {
		t.Errorf(formatStr, "wrong album title", wantData.title, gotData.title)
	}

	if gotData.artist != wantData.artist {
		t.Errorf(formatStr, "wrong artist name", wantData.artist, gotData.artist)
	}

	if !gotData.date.Equal(wantData.date) {
		t.Errorf(formatStr, "wrong release date", wantData.date, gotData.date)
	}

	for i, tag := range gotData.tags {
		if tag != wantData.tags[i] {
			t.Errorf(formatStr, "wrong tags", wantData.tags, gotData.tags)
		}
	}

	if gotData.url != wantData.url {
		t.Errorf(formatStr, "wrong item url", wantData.url, gotData.url)
	}

	// check track data
	for i, track := range wantData.tracks {
		if track.trackNumber != gotData.tracks[i].trackNumber {
			t.Errorf(formatStr, "wrong track number",
				track.trackNumber, gotData.tracks[i].trackNumber)
		}

		if track.title != gotData.tracks[i].title {
			t.Errorf(formatStrLong, "wrong track title",
				"track:", track.trackNumber, "title:", track.title,
				"track:", gotData.tracks[i].trackNumber, "title:", gotData.tracks[i].title)
		}

		if track.duration != gotData.tracks[i].duration {
			t.Errorf(formatStrLong, "wrong track duration",
				"track:", track.trackNumber,
				"duration:", track.duration,
				"track:", gotData.tracks[i].trackNumber,
				"duration:", gotData.tracks[i].duration)
		}

		if track.lyrics != gotData.tracks[i].lyrics {
			t.Errorf(formatStrLong, "wrong lyrics",
				"track:", track.trackNumber,
				"lyrics:\n", track.lyrics,
				"track:", gotData.tracks[i].trackNumber,
				"lyrics:\n", gotData.tracks[i].lyrics)
		}

		// from net.go
		wantURL := getTruncatedURL(track.url)
		gotURL := getTruncatedURL(gotData.tracks[i].url)
		if gotURL != wantURL {
			t.Errorf(formatStrLong, "wrong track url",
				"track:", track.trackNumber, "\ntruncated url:", wantURL,
				"track:", gotData.tracks[i].trackNumber, "\ntruncated url:", gotURL)
		}
	}
}

func init() {
	file, err := os.Open("testdata/album_metada.json")
	if err != nil {
		log.Fatal(err)
	}
	scanner := bufio.NewScanner(file)
	scanner.Scan()
	metaData = scanner.Text()
	scanner.Scan()
	mediaData = scanner.Text()
	if err = file.Close(); err != nil {
		log.Fatal(err)
	}
}
