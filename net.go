package main

import (
	"bufio"
	"errors"
	"fmt"
	"image/jpeg"
	"io"
	"math/rand"
	"net/http"
	"strings"
	"time"
)

// TODO: by default no timeout is set
// TODO: cancel response body readings for unfinished tracks
// will solve a lot of problems

func download(link string, mobile bool, checkDomain bool) (io.ReadCloser, string) {
	window.sendEvent(newDebugMessage(link))
	request, err := http.NewRequest("GET", link, nil)
	if err != nil {
		window.sendEvent(newErrorMessage(err))
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
		window.sendEvent(newErrorMessage(err))
		if strings.Contains(link, "https://") {
			window.sendEvent(newDebugMessage("trying over http://"))
			return download(strings.Replace(link, "https://", "http://", 1),
				mobile, checkDomain)
		}
		return nil, ""
	}
	window.sendEvent(newDebugMessage(response.Status))

	// not all artists are hosted on bandname.bandcamp.com,
	// deal with aliases by reading canonical names from response
	if checkDomain {
		if !strings.Contains(response.Header.Get("Link"),
			"bandcamp.com") {
			window.sendEvent(newErrorMessage(errors.New("response came not from bandcamp.com")))
			response.Body.Close()
			return nil, ""
		}
	}
	return response.Body, response.Header.Get("content-type")
}

func processMediaPage(link string) {
	window.sendEvent(newMessage("fetching media page..."))
	reader, _ := download(link, true, true)
	if reader == nil {
		window.sendEvent(newItem(nil))
		//window.sendEvent(newCoverDownloaded(nil))
		return
	}
	defer reader.Close()
	window.sendEvent(newMessage("parsing..."))

	scanner := bufio.NewScanner(reader)
	// NOTE: might fail here
	// 64k is not enough for all pages apparently
	var buffer []byte
	scanner.Buffer(buffer, 131072)
	var metaDataJSON string
	var mediaDataJSON string
	var isAlbum bool

	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "og:type") {
			if strings.Contains(line, "album") {
				isAlbum = true
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
				window.sendEvent(newErrorMessage(errors.New("unexpected page format")))
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
				window.sendEvent(newErrorMessage(errors.New("unexpected page format")))
				return
			}
			replacer := strings.NewReplacer(`&quot;`, `"`, `&amp;`, `&`, "%2F", "/")
			mediaDataJSON = replacer.Replace(line[start:end])
		}
	}

	var metadata *album
	var err error
	if metaDataJSON != "" || mediaDataJSON != "" {
		if !isAlbum {
			window.sendEvent(newMessage("found track data"))
			metadata, err = parseTrackJSON(metaDataJSON, mediaDataJSON)
		} else {
			window.sendEvent(newMessage("found album data"))
			metadata, err = parseAlbumJSON(metaDataJSON, mediaDataJSON)
		}

		if err == nil {
			window.sendEvent(newItem(metadata))
		} else {
			window.sendEvent(newErrorMessage(err))
		}

	} else {
		window.sendEvent(newErrorMessage(errors.New("unexpected page format")))
	}
}

func downloadMedia(link string, track int) {
	var err error
	key := getTruncatedURL(link)

	if _, ok := cache.get(key); ok {
		window.sendEvent(newTrackDownloaded(key))
		window.sendEvent(newMessage(fmt.Sprintf("playing track %d from cache",
			track+1)))
		return
	}

	window.sendEvent(newMessage(fmt.Sprintf("fetching track %d...", track+1)))
	reader, _ := download(link, false, false)
	if reader == nil {
		return // error should be reported on other end already
	}
	defer reader.Close()
	window.sendEvent(newMessage(fmt.Sprintf("downloading track %d...", track+1)))

	body, err := io.ReadAll(reader)
	if err != nil {
		window.sendEvent(newErrorMessage(err))
		return
	}

	cache.set(getTruncatedURL(link), body)
	window.sendEvent(newTrackDownloaded(key))
	window.sendEvent(newMessage(fmt.Sprintf("track %d downloaded", track+1)))
}

func downloadCover(link string) {
	window.sendEvent(newDebugMessage("fetching album cover..."))
	reader, _ := download(link, false, false)
	if reader == nil {
		window.sendEvent(newCoverDownloaded(nil))
		return
	}
	defer reader.Close()
	window.sendEvent(newDebugMessage("downloading album cover..."))

	img, err := jpeg.Decode(reader)
	if err != nil {
		window.sendEvent(newErrorMessage(err))
		window.sendEvent(newCoverDownloaded(nil))
		return
	}
	window.sendEvent(newDebugMessage("album cover downloaded"))
	window.sendEvent(newCoverDownloaded(img))
}

func processTagPage(args arguments) {
	window.sendEvent(newMessage("fetching tag search page..."))

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
	window.sendEvent(newMessage("parsing..."))

	scanner := bufio.NewScanner(reader)
	var dataBlobJSON string
	// NOTE: might fail here
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
				window.sendEvent(newErrorMessage(errors.New("unexpected page format")))
				return
			}
			replacer := strings.NewReplacer(`&quot;`, `"`, `&amp;`, `&`)
			dataBlobJSON = replacer.Replace(line[start:end])
			break
		}
	}
	if dataBlobJSON == "" {
		window.sendEvent(newErrorMessage(errors.New("unexpected page format")))
		return
	}
	window.sendEvent(newMessage("found data"))

	var urls []string
	if inHighlights {
		urls = parseTagSearchHighlights(dataBlobJSON)
	} else {
		urls = parseTagSearchJSON(dataBlobJSON)
	}

	rand.Seed(time.Now().UnixNano())
	var url string
	if len(urls) > 0 {
		// TODO: remove later
		for i, url := range urls {
			window.sendEvent(newDebugMessage(fmt.Sprint(
				"tag search:", i, " ", url,
			)))
		}
		url = urls[rand.Intn(len(urls))]
		// TODO: remove later
		player.stop()
		player.initPlayer()
		//
		processMediaPage(url)
		return
	}
	window.sendEvent(newMessage("nothing was found"))
}

func getTruncatedURL(link string) string {
	if strings.Contains(link, "?") {
		index := strings.Index(link, "?")
		return link[:index]
	} else {
		return ""
	}
}

// TODO: finish whatever has been started here
// (returning content-type)
