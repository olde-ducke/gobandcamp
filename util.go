package main

import (
	"errors"
	"fmt"
	"strings"
)

// cache key = media url without any parameters
func getTruncatedURL(link string) string {
	if index := strings.Index(link, "?"); index > 0 {
		return link[:index]
	}
	return link
}

func convert(items []item) ([]PlaylistItem, error) {
	var data []PlaylistItem
	for _, i := range items {
		if !i.hasAudio {
			return nil, errors.New(
				fmt.Sprintf("item %s doesn't have media to play",
					i.url))
		}

		for _, t := range i.tracks {
			data = append(data, PlaylistItem{
				Unreleased:  t.unreleasedTrack,
				Streaming:   t.streaming,
				Path:        t.mp3128,
				Title:       t.title,
				Artist:      i.artist,
				Date:        i.albumReleaseDate,
				Tags:        strings.Join(i.tags, " "),
				Album:       i.title,
				AlbumURL:    i.url, // FIXME: build url from art id
				TrackNum:    t.trackNumber,
				TrackArtist: t.artist,
				TrackURL:    t.url,
				ArtPath:     i.artURL,
				TotalTracks: i.totalTracks,
				Duration:    t.duration,
			})
		}
	}

	return data, nil
}
