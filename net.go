package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"image/jpeg"
	"io"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"time"
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
	return &bytesReadSeekCloser{bytes.NewReader(cache.bytes[index])}
}

func download(link string, mobile bool, checkDomain bool) (io.ReadCloser, string) {
	window.sendPlayerEvent(eventDebugMessage(link))
	request, err := http.NewRequest("GET", link, nil)
	if err != nil {
		window.sendPlayerEvent(err)
		return nil, ""
	}
	// pretend that we are Chrome on Win10
	request.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/93.0.4577.82 Safari/537.36")
	// set mobile view, html weights a bit less than desktop version
	if mobile {
		request.Header.Set("Cookie", "mvp=p")
	}

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		// https requests fail here because reasons (real certificate is replacced by expired
		// generic one), only relevant for images at the moment
		window.sendPlayerEvent(err)
		if strings.Contains(link, "https://") {
			window.sendPlayerEvent(eventDebugMessage("trying over http://"))
			return download(strings.Replace(link, "https://", "http://", 1),
				mobile, checkDomain)
		}
		return nil, ""
	}
	window.sendPlayerEvent(eventDebugMessage(response.Status))

	// not all artists are hosted on bandname.bandcamp.com,
	// deal with aliases by reading canonical names from response
	if checkDomain {
		if !strings.Contains(response.Header.Get("Link"),
			"bandcamp.com") {
			window.sendPlayerEvent(errors.New("response came not from bandcamp.com"))
			response.Body.Close()
			return nil, ""
		}
	}
	return response.Body, response.Header.Get("content-type")
}

func processMediaPage(link string, model *playerModel) {
	window.sendPlayerEvent("fetching media page...")
	reader, _ := download(link, true, true)
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
		} else if strings.Contains(line, "data-cart=") {
			start := strings.Index(line, "data-cart=")
			start += 10
			end := strings.Index(line[start:], "\"")
			end += start
			if start == -1 || end == -1 || end < start {
				window.sendPlayerEvent(errors.New("unexpected page format"))
				return
			}
			replacer := strings.NewReplacer(`&quot;`, `"`, `&amp;`, `&`, "%2F", "/")
			mediaDataJSON = replacer.Replace(line[start:end])
		}
	}

	if metaDataJSON != "" || mediaDataJSON != "" {
		if !album {
			window.sendPlayerEvent("found track data")
			model.metadata = parseTrackJSON(metaDataJSON, mediaDataJSON)
			window.sendPlayerEvent(eventNewItem(0))
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
	var err error
	track := player.currentTrack
	if link == "" {
		window.sendPlayerEvent(fmt.Sprintf("track %d not available for streaming",
			track+1))
		return
	}
	if _, ok := cache.bytes[track]; ok {
		window.sendPlayerEvent(eventTrackDownloader(track))
		window.sendPlayerEvent(fmt.Sprintf("playing track %d from cache",
			track+1))
		return
	}
	cache.mu.Lock()
	defer cache.mu.Unlock()
	window.sendPlayerEvent(fmt.Sprintf("fetching track %d...", track+1))
	reader, _ := download(link, false, false)
	if reader == nil {
		return // error should be reported on other end already
	}
	defer reader.Close()
	window.sendPlayerEvent(fmt.Sprintf("downloading track %d...", track+1))

	cache.bytes[track], err = io.ReadAll(reader)
	if err != nil {
		window.sendPlayerEvent(err)
		return
	}
	window.sendPlayerEvent(eventTrackDownloader(track))
	window.sendPlayerEvent(fmt.Sprintf("track %d downloaded", track+1))
	// TODO: replace in-memory cache with saving on disk
	// SDL seems to be able to only open local files
	//_, err = io.Copy(file, response.Body)
}

func downloadCover(link string, model *artModel) {
	window.sendPlayerEvent(eventDebugMessage("fetching album cover..."))
	reader, _ := download(link, false, false)
	if reader == nil {
		window.sendPlayerEvent(eventCoverDownloader(-1))
		return
	}
	defer reader.Close()
	window.sendPlayerEvent(eventDebugMessage("downloading album cover..."))

	img, err := jpeg.Decode(reader)
	if err != nil {
		window.sendPlayerEvent(err)
		window.sendPlayerEvent(eventCoverDownloader(-1))
		return
	}
	model.cover = img
	window.sendPlayerEvent(eventDebugMessage("album cover downloaded"))
	window.sendPlayerEvent(eventCoverDownloader(0))
}

func processTagPage(args arguments) {
	window.sendPlayerEvent("fetching tag search page...")

	sbuilder := strings.Builder{}
	defer sbuilder.Reset()
	fmt.Fprint(&sbuilder, "https://bandcamp.com/tag/", args.tags[0], "?tab=all_releases")

	if len(args.tags) > 1 {
		fmt.Fprint(&sbuilder, "&t=")
		for i := 1; i < len(args.tags); i++ {
			if i == 1 {
				fmt.Fprint(&sbuilder, args.tags[i])
				continue
			}
			fmt.Fprint(&sbuilder, "%2C", args.tags[i])
		}
	}

	var inHighlights bool
	if args.sort != "" && args.sort != "highlights" {
		fmt.Fprint(&sbuilder, "&s=", args.sort)
	} else if args.sort == "highlights" {
		inHighlights = true
	}

	if args.format != "" {
		fmt.Fprint(&sbuilder, "&f=", args.format)
	}

	reader, _ := download(sbuilder.String(), false, true)
	if reader == nil {
		return
	}
	defer reader.Close()
	window.sendPlayerEvent("parsing...")

	scanner := bufio.NewScanner(reader)
	var dataBlobJSON string
	// 64k is not enough for these pages
	var buffer []byte
	scanner.Buffer(buffer, 1048576)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "data-blob=") {
			start := strings.Index(line, `data-blob=`)
			start += 11
			end := strings.Index(line[start:], "></div>")
			end += start
			end--
			if start == -1 || end == -1 || end < start {
				window.sendPlayerEvent(errors.New("unexpected page format"))
				return
			}
			replacer := strings.NewReplacer(`&quot;`, `"`, `&amp;`, `&`)
			dataBlobJSON = replacer.Replace(line[start:end])
			break
		}
	}
	if dataBlobJSON == "" {
		window.sendPlayerEvent(errors.New("unexpected page format"))
		return
	}
	window.sendPlayerEvent("found data")

	var urls []string
	if inHighlights {
		urls = parseTagSearchHighlights(dataBlobJSON)
	} else {
		urls = parseTagSearchJSON(dataBlobJSON)
	}

	file, err := os.Create("temp.html")
	checkFatalError(err)
	defer file.Close()

	rand.Seed(time.Now().UnixNano())
	var url string
	if urls != nil {
		for _, url := range urls {
			file.WriteString(url + "\n")
		}
		url = urls[rand.Intn(len(urls))]
		player.stop()
		player.initPlayer()
		processMediaPage(url, window.playerM)
		return
	}
	window.sendPlayerEvent("nothing was found")
}

// TODO: finish whatever has been started here
// (returning content-type)
