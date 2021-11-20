package main

import (
	"bufio"
	"log"
	"os"
	"testing"
)

func TestParseAlbumJSON(t *testing.T) {

	file, err := os.Open("testdata/album_metada.json")
	if err != nil {
		log.Fatal(err)
	}
	scanner := bufio.NewScanner(file)
	scanner.Scan()
	metadata := scanner.Text()

	/*testdata := &album{
		imageSrc:    "https://example.com/gopher_255644.png",
		title:       "album_name_test",
		artist:      "artist_name_test",
		date:        "01 jan 1970",
		url:         "https://example.com/gopher.html",
		tags:        "gopher music png",
		totalTracks: 3,
		tracks: []track{
			{
				trackNumber: 1,
				title:       "testing",
				duration:    202.241,
				lyrics:      "testing\r\nlyrics\r\non\r\nfirst\r\ntrack",
				url:         "https://example.com/7646382/gopher-1.wav",
			},
			{
				trackNumber: 2,
				title:       "track",
				duration:    0.0,
				lyrics:      "testing\r\nlyrics\r\non\r\nsecond\r\ntrack",
				url:         "",
			},
			{
				trackNumber: 3,
				title:       "titles",
				duration:    836.75,
				lyrics:      "",
				url:         "https://example.com/gopher-1.wav?t=eufhiuhaushdpoaihfpaosjd",
			},
		},
	} */

	//formatstring := "imageSrc: %s title: %s artist: %s date: %s tags: %s totalTracks: %d"
	gotdata, err := parseTrAlbumJSON(metadata, "", true)
	if err == nil || gotdata != nil {
		t.Fatalf("\nwant: <nil>, error,\n got: %v, %s", gotdata, err)
	}

	/*want := fmt.Sprintf(formatstring,
		testdata.imageSrc,
		testdata.title,
		testdata.artist,
		testdata.date,
		testdata.tags,
		testdata.totalTracks)
	got := fmt.Sprintf(formatstring,
		gotdata.imageSrc,
		gotdata.title,
		gotdata.artist,
		gotdata.date,
		gotdata.tags,
		gotdata.totalTracks)
	if want != got || err != nil {
		t.Fatalf("parseAlbumJSON(string string)\ngot:\n%q,\nwant:\n%q\n", got, want)
	} */
}
