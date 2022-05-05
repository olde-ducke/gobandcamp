package main

import (
	"errors"
	"strings"
)

// cache key = media url without any parameters
func getTruncatedURL(link string) string {
	if index := strings.Index(link, "?"); index > 0 {
		return link[:index]
	}
	return link
}

func convert(items ...item) ([]PlaylistItem, error) {
	if len(items) == 0 {
		return nil, errors.New("nothing to add to playlist")
	}

	var data []PlaylistItem
	for _, i := range items {
		if !i.hasAudio {
			continue
			// TODO: report this
			// return nil, fmt.Errorf(
			//	"item %s doesn't have media to play", i.url)
		}

		for _, t := range i.tracks {
			path := t.mp3128
			if t.mp3v0 != "" {
				path = t.mp3v0
			}

			data = append(data, PlaylistItem{
				Unreleased:  t.unreleasedTrack,
				Streaming:   t.streaming == 1,
				Path:        path,
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

	if len(data) == 0 {
		return nil, errors.New("nothing was added to playlist")
	}

	return data, nil
}
