package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"image/jpeg"
	"io"
	"net/http"
	"strings"
)

// response.Body doesn't implement Seek() method
// beep isn't bothered by this, but trying to
// call Seek() will fail since Len() will always return 0
// by using bytes.Reader and implementing empty Close() method
// we get io.ReadSeekCloser, which satisfies requirements of beep streamers
// (need ReadCloser) and implements Seek() method

type bytesReadSeekCloser struct {
	*bytes.Reader
}

func (c bytesReadSeekCloser) Close() error {
	return nil
}

func wrapInRSC(index int) *bytesReadSeekCloser {
	return &bytesReadSeekCloser{bytes.NewReader(cache[index])}
}

func download(link string, mobile bool, checkDomain bool) io.ReadCloser {
	request, err := http.NewRequest("GET", link, nil)
	if err != nil {
		window.sendPlayerEvent(err)
		return nil
	}
	// pretend that we are Chrome on Win10
	request.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/93.0.4577.82 Safari/537.36")
	// set mobile view, html weights a bit less than desktop version
	if mobile {
		request.Header.Set("Cookie", "mvp=p")
	}

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		// images requests over https keep failing for me
		if !strings.Contains(link, "http://") {
			window.sendPlayerEvent(err)
			window.sendPlayerEvent("trying over http://")
			return download(strings.Replace(link, "https://", "http://", 1),
				mobile, checkDomain)
		}
		return nil
	}
	window.sendPlayerEvent(response.Status)

	// not all artists are hosted on bandname.bandcamp.com,
	// deal with aliases by reading canonical names from response
	if checkDomain {
		if !strings.Contains(response.Header.Get("Link"),
			"bandcamp.com") {
			window.sendPlayerEvent(errors.New("response came not from bandcamp.com"))
			response.Body.Close()
			return nil
		}
	}
	return response.Body
}

func processMediaPage(link string, mobile bool, model *playerModel) {
	window.sendPlayerEvent("fetching page...")
	reader := download(link, mobile, true)
	if reader == nil {
		window.sendPlayerEvent(eventNewItem(-1))
		return
	}
	defer reader.Close()
	window.sendPlayerEvent("parsing...")

	scanner := bufio.NewScanner(reader)
	var metaDataJSON string
	var mediaDataJSON string
	var album bool

	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "og:type") {
			if strings.Contains(line, "album") {
				album = true
			}
		} else if strings.Contains(line, "application/ld+json") {
			scanner.Scan()
			metaDataJSON = scanner.Text()
		} else if strings.Contains(line, "data-tralbum=") {
			start := strings.Index(line, "data-tralbum=")
			start += 14
			end := strings.Index(line[start:], "\"")
			end += start
			if start == -1 || end == -1 || end < start {
				window.sendPlayerEvent(errors.New("unexpected page format"))
				return
			}
			replacer := strings.NewReplacer(`&quot;`, `"`, `&amp;`, `&`)
			mediaDataJSON = replacer.Replace(line[start:end])
		}
	}

	if metaDataJSON != "" || mediaDataJSON != "" {
		if !album {
			window.sendPlayerEvent("found track data (not implemented)")
			window.sendPlayerEvent(eventCoverDownloader(-1))
			window.sendPlayerEvent(eventNewItem(-1))
		} else {
			window.sendPlayerEvent("found album data")
			model.metadata = parseAlbumJSON(metaDataJSON, mediaDataJSON)
			window.sendPlayerEvent(eventNewItem(0))
		}
	} else {
		window.sendPlayerEvent(errors.New("unexpected page format"))
	}
}

func downloadMedia(link string) {
	track := player.currentTrack
	window.sendPlayerEvent(fmt.Sprintf("fetching track %d...", track+1))
	if link == "" {
		window.sendPlayerEvent(fmt.Sprintf("track %d not available for streaming",
			track+1))
		return
	}
	if _, ok := cache[track]; ok {
		window.sendPlayerEvent(eventTrackDownloader(track))
		window.sendPlayerEvent(fmt.Sprintf("playing track %d from cache",
			track+1))
		return
	}
	reader := download(link, false, false)
	if reader == nil {
		return
	}
	defer reader.Close()
	window.sendPlayerEvent(fmt.Sprintf("downloading track %d...", track+1))

	body, err := io.ReadAll(reader)
	if err != nil {
		window.sendPlayerEvent(err)
		return
	}
	cache[track] = body
	window.sendPlayerEvent(eventTrackDownloader(track))
	window.sendPlayerEvent(fmt.Sprintf("track %d downloaded", track+1))
	// TODO: replace in-memory cache with saving on disk
	// SDL seems to be able to only open local files
	//_, err = io.Copy(file, response.Body)
}

func downloadCover(link string, model *artModel) {
	window.sendPlayerEvent("fetching album cover...")
	reader := download(link, false, false)
	if reader == nil {
		window.sendPlayerEvent(eventCoverDownloader(-1))
		return
	}
	defer reader.Close()
	window.sendPlayerEvent("downloading album cover...")

	img, err := jpeg.Decode(reader)
	if err != nil {
		window.sendPlayerEvent(err)
		window.sendPlayerEvent(eventCoverDownloader(-1))
		return
	}
	model.cover = img
	window.sendPlayerEvent("album cover downloaded")
	window.sendPlayerEvent(eventCoverDownloader(0))
}

// TODO: bandcamp tag request and parser
