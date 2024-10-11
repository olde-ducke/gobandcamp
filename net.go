package main

import (
	"bufio"
	"bytes"
	"compress/flate"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"strings"
	"time"
)

var client = http.Client{Timeout: 120 * time.Second}

// TODO: cancel response body readings for unfinished tracks
// will solve a lot of problems
// TODO: maybe it is a good idea to check domain everytime, just in case?
func download(link string, mobile bool, checkDomain bool) (io.ReadCloser, string) {
	window.sendEvent(newDebugMessage(link))
	request, err := http.NewRequest("GET", link, nil)
	if err != nil {
		window.sendEvent(newErrorMessage(err))
		return nil, ""
	}
	// pretend that we are Chrome on Win10
	request.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 12_0_1) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/95.0.4638.69 Safari/537.36")
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
			window.sendEvent(newDebugMessage(err.Error() + "; trying over http://"))
			return download(strings.Replace(link, "https://", "http://", 1),
				mobile, checkDomain)
		}
		window.sendEvent(newErrorMessage(err))
		return nil, ""
	}
	window.sendEvent(newDebugMessage(response.Status))

	// not all artists are hosted on bandname.bandcamp.com,
	// deal with aliases by reading canonical names from response
	if checkDomain {
		if !strings.Contains(response.Header.Get("Link"),
			"bandcamp.com") {
			window.sendEvent(newErrorMessage(errors.New("response came not from bandcamp.com")))
			window.sendEvent(newDebugMessage(fmt.Sprint(response)))
			response.Body.Close()
			return nil, ""
		}
	}
	return response.Body, response.Header.Get("content-type")
}

func processMediaPage(link string) {
	defer wg.Done()
	window.sendEvent(newMessage("fetching media page..."))
	reader, _ := download(link, false, true)
	if reader == nil {
		window.sendEvent(newItem(nil))
		return
	}
	defer reader.Close()
	window.sendEvent(newMessage("parsing..."))

	scanner := bufio.NewScanner(reader)
	// NOTE: might fail here
	// 64k is not enough for all pages apparently
	// failed on a page with 43 tracks
	var buffer []byte
	scanner.Buffer(buffer, 131072)
	var metaDataJSON string
	var mediaDataJSON string
	var err error
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
		window.sendEvent(newErrorMessage(err))
		return
	}

	if metaDataJSON != "" || mediaDataJSON != "" {
		if !isAlbum {
			window.sendEvent(newMessage("found track data"))
		} else {
			window.sendEvent(newMessage("found album data"))
		}

		metadata, err := parseTrAlbumJSON(metaDataJSON, mediaDataJSON, isAlbum)

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
	defer wg.Done()
	var err error
	key := getTruncatedURL(link)

	// TODO: move this check to upper level?
	if _, ok := cache.get(key); ok {
		window.sendEvent(newTrackDownloaded(key))
		window.sendEvent(newMessage(fmt.Sprintf("playing track %d from cache",
			track+1)))
		return
	}
	window.sendEvent(newMessage(fmt.Sprintf("fetching track %d...", track+1)))
	// NOTE: media location suggests that there is always only mp3 files on server
	// for now ignore type of media
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
	defer wg.Done()
	window.sendEvent(newDebugMessage("fetching album cover..."))
	reader, format := download(link, false, false)
	if reader == nil {
		window.sendEvent(newCoverDownloaded(nil, ""))
		return
	}
	defer reader.Close()
	window.sendEvent(newDebugMessage("downloading album cover..."))

	var img image.Image
	var err error
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
		window.sendEvent(newErrorMessage(err))
		window.sendEvent(newCoverDownloaded(nil, ""))
		return
	}

	window.sendEvent(newDebugMessage("album cover downloaded"))
	window.sendEvent(newCoverDownloaded(img, link))
}

func processTagPage(args arguments) {
	defer wg.Done()
	window.sendEvent(newMessage("fetching data..."))

	result, err := makeDiscoverRequest(&DiscoverRequest{
		CategoryID:         args.format,
		Cursor:             "*",
		GeonameID:          0,
		IncludeResultTypes: []string{"a"},
		Size:               60,
		Slice:              args.sort,
		TagNormNames:       args.tags,
	})
	if err != nil {
		window.sendEvent(newErrorMessage(err))
		return
	}

	// FIXME: not sure if either can be trusted
	if result.BatchResultCount == 0 || result.ResultCount == 0 ||
		len(result.Results) == 0 {
		window.sendEvent(newMessage("nothing was found"))
		return
	}

	window.sendEvent(newMessage("found data"))
	window.sendEvent(newTagSearch(result))
}

func extractJSON(prefix, line, suffix string) (string, error) {
	start := strings.Index(line, prefix)
	start += len(prefix)
	end := strings.Index(line[start:], suffix)
	end += start
	if start == -1 || end == -1 || end < start {
		return "", errors.New("unexpected page format")
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

func getAdditionalResults(req *DiscoverRequest) {
	defer wg.Done()
	window.sendEvent(newMessage("pulling additional results..."))

	result, err := makeDiscoverRequest(req)
	if err != nil {
		window.sendEvent(newAdditionalTagSearch(nil))
		window.sendEvent(newErrorMessage(err))
		return
	}

	if result.BatchResultCount == 0 || result.ResultCount == 0 ||
		len(result.Results) == 0 {
		window.sendEvent(newAdditionalTagSearch(nil))
		window.sendEvent(newMessage("nothing was found"))
		return
	}

	window.sendEvent(newAdditionalTagSearch(result))
}

func makeDiscoverRequest(req *DiscoverRequest) (*DiscoverResult, error) {
	buf := bytes.Buffer{}
	defer buf.Reset()

	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)

	if err := enc.Encode(&req); err != nil {
		return nil, fmt.Errorf("failed to encode request: %w", err)
	}

	window.sendEvent(newDebugMessage(req.String()))

	request, err := http.NewRequest("POST",
		"https://bandcamp.com/api/discover/1/discover_web", &buf)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	request.Header.Set("Accept", "*/*")
	request.Header.Set("Accept-Encoding", "gzip, deflate")
	request.Header.Set("Content-Type", "application/json; charset=UTF-8")
	request.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 12_0_1) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/95.0.4638.69 Safari/537.36")

	resp, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("got unexpected response: %s\n%w", resp.Status, err)
		}

		return nil, fmt.Errorf("got unexpected response: %s\n%s", resp.Status, data)
	}

	result := &DiscoverResult{
		Request: req,
	}

	var (
		dec      *json.Decoder
		encoding string
	)

	if len(resp.Header.Values("Content-Encoding")) > 0 {
		encoding = resp.Header.Values("Content-Encoding")[0]
	}

	switch encoding {
	case "gzip":
		gr, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to decode gzip body: %w", err)
		}
		defer gr.Close()
		dec = json.NewDecoder(gr)

	case "deflate":
		dr := flate.NewReader(resp.Body)
		defer dr.Close()
		dec = json.NewDecoder(dr)

	default:
		dec = json.NewDecoder(resp.Body)
	}

	// dec.DisallowUnknownFields()

	if err := dec.Decode(result); err != nil {
		return nil, fmt.Errorf("failed to read response: %w\n%+v", err, resp.Header)
	}

	if result.ErrorType != "" {
		return nil, fmt.Errorf("got api error: %s request: %s",
			result.ErrorType, req)
	}

	return result, nil
}
