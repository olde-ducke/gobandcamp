package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/97.0.4692.71 Safari/537.36"

var originError = errors.New("response came not from bandcamp.com")
var unexpectedError = errors.New("unexpected page format")

var client = http.Client{Timeout: 120 * time.Second}

type info struct {
	s string
}

func (i *info) String() string {
	return i.s
}

func newInfoMessage(text string) *info {
	return &info{text}
}

// TODO: cancel response body readings for unfinished tracks
// will solve a lot of problems
// TODO: maybe it is a good idea to check domain everytime, just in case?
func download(link string, mobile bool, checkDomain bool) (io.ReadCloser, string, error) {
	request, err := http.NewRequest("GET", link, nil)
	if err != nil {
		return nil, "", err
	}
	// pretend that we are Chrome on Win10
	request.Header.Set("User-Agent", userAgent)
	// set mobile view, html weights a bit less than desktop version
	if mobile {
		request.Header.Set("Cookie", "mvp=p")
	}

	response, err := client.Do(request)
	if err != nil {
		// https requests fail here because reasons (real certificate
		// is replacced by expired generic one), only relevant for
		// images at the moment
		// basically try http instead of https, and don't report error
		if strings.Contains(link, "https://") {
			// window.sendEvent(newDebugMessage(err.Error() + "; trying over http://"))
			return download(strings.Replace(link, "https://", "http://", 1),
				mobile, checkDomain)
		}
		return nil, "", err
	}
	// window.sendEvent(newDebugMessage(response.Status))

	// not all artists are hosted on bandname.bandcamp.com,
	// deal with aliases by reading canonical names from response
	if checkDomain {
		if !strings.Contains(response.Header.Get("Link"),
			"bandcamp.com") {
			// window.sendEvent(newDebugMessage(fmt.Sprint(response)))
			response.Body.Close()
			return nil, "", originError
		}
	}
	return response.Body, response.Header.Get("content-type"), nil
}

func processMediaPage(link string, message chan<- interface{}) {
	defer wg.Done()
	message <- newInfoMessage("fetching media page...")
	message <- link

	reader, _, err := download(link, false, true)
	if err != nil {
		// window.sendEvent(newItem(nil))
		message <- "here nil data must be sent, but why exactly?"
		message <- err
		return
	}
	defer reader.Close()
	message <- newInfoMessage("parsing...")

	scanner := bufio.NewScanner(reader)
	// NOTE: might fail here
	// 64k is not enough for all pages apparently
	// failed on a page with 43 tracks
	var buffer []byte
	scanner.Buffer(buffer, 131072)
	var metaDataJSON string
	var mediaDataJSON string
	var isAlbum bool

	// TODO: expects only album/track pages and artist page with pinned item
	// if artist page is discography, will try to parse it as album/track
	// doesn't crash, but doesn't need to get there in the first place
	// NOTE: 5ccc945faa4b3243c4ab9f71ec53ed0c9a5c7df7 - absolutely zero idea
	// what was here before and why, probably there was a reason for that
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "og:type") {
			if strings.Contains(line, "album") {
				isAlbum = true
			}
		} else if strings.Contains(line, "application/ld+json") {
			scanner.Scan()
			metaDataJSON = scanner.Text()
		} else if prefix := `data-tralbum="`; strings.Contains(line, prefix) {
			mediaDataJSON, err = extractJSON(prefix, line, `"`)
		}
	}

	if err != nil {
		message <- err
		return
	}

	if metaDataJSON != "" || mediaDataJSON != "" {
		if !isAlbum {
			message <- newInfoMessage("found track data")
		} else {
			message <- newInfoMessage("found album data")
		}

		_, err := parseTrAlbumJSON(metaDataJSON, mediaDataJSON, isAlbum)
		if err != nil {
			message <- err
		} else {
			// window.sendEvent(newItem(metadata))
			message <- "here data should be sent, but alas"
		}

	} else {
		message <- unexpectedError
	}
}

func downloadMedia(link string, track int, message chan<- interface{}) {
	defer wg.Done()
	var err error
	key := getTruncatedURL(link)

	// TODO: move this check to upper level?
	if _, ok := cache.get(key); ok {
		// window.sendEvent(newTrackDownloaded(key))
		message <- newInfoMessage(fmt.Sprintf("playing track %d from cache",
			track+1))
		return
	}
	message <- newInfoMessage(fmt.Sprintf("fetching track %d...", track+1))
	// NOTE: media location suggests that there is always only mp3 files on server
	// for now ignore type of media
	message <- link
	reader, _, err := download(link, false, false)
	if err != nil {
		message <- err
		return
	}
	defer reader.Close()
	message <- newInfoMessage(fmt.Sprintf("downloading track %d...", track+1))

	body, err := io.ReadAll(reader)
	if err != nil {
		message <- err
		return
	}

	cache.set(getTruncatedURL(link), body)
	// window.sendEvent(newTrackDownloaded(key))
	message <- "here new track should be sent, but alas"
	message <- newInfoMessage(fmt.Sprintf("track %d downloaded", track+1))
}

func downloadCover(link string, message chan<- interface{}) {
	defer wg.Done()
	message <- "fetching album cover..."
	message <- link
	reader, format, err := download(link, false, false)
	if err != nil {
		// window.sendEvent(newCoverDownloaded(nil, ""))
		message <- err
		message <- "here empty album cover should be sent, but alas"
		return
	}
	defer reader.Close()
	message <- "downloading album cover..."

	var img image.Image
	switch format {

	case "image/jpeg":
		img, err = jpeg.Decode(reader)

	// in case there is png somewhere for whatever reason
	case "image/png":
		img, err = png.Decode(reader)

	default:
		img, err = nil, errors.New("unexpected image format")
	}

	if err != nil {
		message <- err
		// window.sendEvent(newCoverDownloaded(nil, ""))
		message <- "here empty album cover should be sent, but alas, here image dimensions:" +
			fmt.Sprint(img.Bounds())
		return
	}

	message <- "album cover downloaded"
	// window.sendEvent(newCoverDownloaded(img, link))
	message <- "here album cover should be sent, but alas"
}

func processTagPage(args *arguments, message chan<- interface{}) {
	defer wg.Done()
	message <- newInfoMessage("fetching tag search page...")

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

	reader, _, err := download(sbuilder.String(), false, true)
	if err != nil {
		message <- err
		return
	}
	defer reader.Close()
	message <- newInfoMessage("parsing...")

	scanner := bufio.NewScanner(reader)
	var dataBlobJSON string
	// NOTE: might fail here
	// 64k is not enough for these pages
	// buffer with size 1048576 reads json with 178 items
	var buffer []byte
	scanner.Buffer(buffer, 1048576)
	for scanner.Scan() {
		line := scanner.Text()
		if prefix := `data-blob="`; strings.Contains(line, prefix) {
			dataBlobJSON, err = extractJSON(prefix, line, `"`)
			break
		}
	}
	if err != nil {
		message <- err
		return
	}
	message <- newInfoMessage("found data")

	results, err := parseTagSearchJSON(dataBlobJSON, inHighlights)
	if err != nil {
		message <- err
		return
	}

	if results == nil || len(results.Items) == 0 {
		message <- newInfoMessage("nothing was found")
		return
	}
	// window.sendEvent(newTagSearch(results))
	message <- "here search results should be sent, but alas"
}

func extractJSON(prefix, line, suffix string) (string, error) {
	start := strings.Index(line, prefix)
	start += len(prefix)
	end := strings.Index(line[start:], suffix)
	end += start
	if start == -1 || end == -1 || end < start {
		return "", unexpectedError
	}
	replacer := strings.NewReplacer(`&quot;`, `"`, `&amp;`, `&`, `&#39;`, `'`)
	return replacer.Replace(line[start:end]), nil
}

// cache key = media url without any parameters
func getTruncatedURL(link string) string {
	if strings.Contains(link, "?") {
		index := strings.Index(link, "?")
		return link[:index]
	} else {
		return ""
	}
}

func getAdditionalResults(result *Result, message chan<- interface{}) {
	defer wg.Done()
	message <- newInfoMessage("pulling additional results...")
	jsonString := "{\"filters\":" + result.filters + ",\"page\":" +
		strconv.Itoa(result.page) + "}"
	buffer := bytes.NewBuffer([]byte(jsonString))

	request, err := http.NewRequest("POST",
		"https://bandcamp.com/api/hub/2/dig_deeper", buffer)
	if err != nil {
		// window.sendEvent(newAdditionalTagSearch(nil))
		message <- "empty search results must be sent, but alas"
		message <- err
		return
	}
	// pretend that we are Chrome on Win10
	request.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 12_0_1) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/95.0.4638.69 Safari/537.36")

	response, err := client.Do(request)
	if err != nil {
		// window.sendEvent(newAdditionalTagSearch(nil))
		message <- "empty search results must be sent, but alas"
		message <- err
		return
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		// window.sendEvent(newAdditionalTagSearch(nil))
		message <- "empty search results must be sent, but alas"
		message <- err
		return
	}

	_, err = extractResults(body)
	if err != nil {
		// window.sendEvent(newAdditionalTagSearch(nil))
		message <- "empty search results must be sent, but alas"
		message <- err
		return
	}
	// window.sendEvent(newAdditionalTagSearch(additionalResult))
	message <- "additional search results must be sent, but alas"
}
