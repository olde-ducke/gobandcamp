package main

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/html"
)

const userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/97.0.4692.71 Safari/537.36"
const chunkSize = 2048

var originError = errors.New("response came not from bandcamp.com")
var unexpectedError = errors.New("unexpected page format")

var client = http.Client{Timeout: 120 * time.Second}

// cache key = media url without any parameters
func getTruncatedURL(link string) string {
	if strings.Contains(link, "?") {
		index := strings.Index(link, "?")
		return link[:index]
	} else {
		return ""
	}
}

type options struct {
	ctx    context.Context
	url    string
	method string
	body   io.Reader
	// headers map[string]string
}

func makeRequest(ops *options, f func(*http.Response, error) error) error {
	request, err := http.NewRequest(ops.method, ops.url, ops.body)
	if err != nil {
		return err
	}

	// request.Header.Set("Cookie", "mvp=p")
	// TODO: set cookie properly and not directly as header?
	// but not sure what's the point doing it that way, header
	// itself has only value, and setting to mobile view
	// is situational, since it breaks tag searching
	// at least in highlights, which are not accessible
	// through api, regular search on mobile is also
	// different though
	// Domain: can be .bandcamp.com or whatever it
	// actually is
	// HostOnly: false
	// HttpOnly: false
	// Path: /
	// SameSite: Lax
	// Secure: false
	// maxage = 90 days
	// TODO: make proper chrome-like headers? there's
	// absolutely no point in setting only user-agent,
	// either properly mimic chrome, or do nothing at all

	ch := make(chan error, 1)
	request = request.WithContext(ops.ctx)
	go func() { ch <- f(client.Do(request)) }()
	select {
	case <-ops.ctx.Done():
		<-ch
		return ops.ctx.Err()
	case err := <-ch:
		return err
	}
}

func processmediapage(ctx context.Context, link string, dbg, msg func(string)) ([]item, error) {
	dbg(link)
	msg("fetching")
	ops := options{
		ctx:    ctx,
		url:    link,
		method: "GET",
	}

	var items []item
	err := makeRequest(&ops, func(response *http.Response, err error) error {
		if err != nil {
			return err
		}
		defer response.Body.Close()

		if response.StatusCode > 200 {
			return errors.New(response.Status)
		}
		msg(response.Status)

		// check canonical name in response header
		// must be artist.bandcamp.com
		if !strings.Contains(response.Header.Get("link"), "bandcamp.com") {
			return originError
		}

		doc, err := html.Parse(response.Body)
		if err != nil {
			return err
		}

		itemType, ok := getValWithAttr(doc, &html.Attribute{
			Key: "property",
			Val: "og:type",
		}, "meta", "content")
		if !ok {
			dbg("failed to parse page type")
			return unexpectedError
		}

		switch itemType {
		case "album":
			msg("found album data")

		case "song":
			msg("found track data")

		case "band":
			msg("found discography")
			// TODO: finish
			node, ok := getNodeWithAttr(doc, &html.Attribute{
				Key: "id",
				Val: "music-grid",
			}, "ol")
			if !ok {
				return unexpectedError
			}
			result := getValues(node, "a", "href")
			if result != nil {
				url, err := url.Parse(link)
				// FIXME: unecessarry?
				if err != nil {
					return err
				}

				items = make([]item, len(result))
				var wg sync.WaitGroup
				for i, path := range result {
					url.Path = path
					wg.Add(1)
					go func(n int, u string) {
						defer wg.Done()
						// FIXME: messy
						res, err := processmediapage(ctx, u, dbg, func(string) {})
						if err != nil {
							dbg(err.Error())
						} else if len(res) > 1 {
							dbg("unexpected results: more than 1 album")
						} else if len(res) > 0 {
							items[n] = res[0]
							dbg("done with " + u)
						}
					}(i, url.String())
				}
				wg.Wait()

				if len(items) > 0 {
					return nil
				}
			}
			return unexpectedError

		default:
			return unexpectedError
		}

		mediadata, ok := getValWithAttr(doc, &html.Attribute{
			Key: "type",
			Val: "text/javascript",
		}, "script", "data-tralbum")
		if !ok {
			dbg("failed to parse mediadata")
			return unexpectedError
		}

		metadata, ok := getTextWithAttr(doc, &html.Attribute{
			Key: "type",
			Val: "application/ld+json",
		}, "script")
		if !ok {
			return unexpectedError
		}

		buf, err := parseTrAlbumJSON(metadata, mediadata)
		if err != nil {
			return err
		}
		items = make([]item, 1)
		items[0] = *buf

		return nil
	})

	return items, err
}

// FIXME: might be not very effective
// write similar with tokenizer and compare?
func getAttrVal(node *html.Node, attrKey string) (string, bool) {
	for _, a := range node.Attr {
		if a.Key == attrKey {
			return a.Val, true
		}
	}
	return "", false
}

func hasAttr(node *html.Node, attr *html.Attribute) bool {
	if node.Type == html.ElementNode {
		val, ok := getAttrVal(node, attr.Key)
		if ok && val == attr.Val {
			return true
		}
	}
	return false
}

// returns first encountered node with tag and attribute
func getNodeWithAttr(node *html.Node, attr *html.Attribute, tag string) (*html.Node, bool) {
	if node.Type == html.ElementNode && node.Data == tag {
		if hasAttr(node, attr) {
			return node, true
		}
	}

	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if result, ok := getNodeWithAttr(child, attr, tag); ok {
			return result, ok
		}
	}

	return nil, false
}

// returns first encountered node with tag
func getNodeWithTag(node *html.Node, tag string) (*html.Node, bool) {
	if node.Type == html.ElementNode && node.Data == tag {
		return node, true
	}

	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if result, ok := getNodeWithTag(child, tag); ok {
			return result, ok
		}
	}

	return nil, false
}

// extracts all tag attributes from inner tags
// FIXME: will return only class1:
// <div class="class1">
// 	<div class="class2">
// 	</div>
// </div>
func getValues(node *html.Node, tag, key string) []string {
	if node.Type != html.ElementNode {
		return nil
	}

	var result []string
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if next, ok := getNodeWithTag(child, tag); ok {
			if val, ok := getAttrVal(next, key); ok {
				result = append(result, val)
			}
		}
	}
	return result
}

func getValWithAttr(node *html.Node, attr *html.Attribute, tag, target string) (string, bool) {
	if node.Type == html.ElementNode && node.Data == tag {
		if hasAttr(node, attr) {
			return getAttrVal(node, target)
		}
	}

	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if val, ok := getValWithAttr(child, attr, tag, target); ok {
			return val, ok
		}
	}

	return "", false
}

func getTextWithAttr(node *html.Node, attr *html.Attribute, tag string) (string, bool) {
	if node.Type == html.ElementNode && node.Data == tag {
		if hasAttr(node, attr) {
			if child := node.FirstChild; child != nil {
				if child.Type == html.TextNode {
					return child.Data, true
				}
			}
		}
	}

	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if val, ok := getTextWithAttr(child, attr, tag); ok {
			return val, ok
		}
	}

	return "", false
}

// TODO: return data format
func downloadmedia(ctx context.Context, link string, dbg, msg func(string)) ([]byte, error) {
	dbg(link)
	msg("fetching")
	ops := options{
		ctx:    ctx,
		url:    link,
		method: "GET",
	}
	var data []byte
	// var dataType string
	err := makeRequest(&ops, func(response *http.Response, err error) error {
		if err != nil {
			return err
		}
		defer response.Body.Close()

		if response.StatusCode > 200 {
			return errors.New(response.Status)
		}
		msg(response.Status)

		lengthValue := response.Header.Get("Content-Length")
		length, err := strconv.Atoi(lengthValue)
		if err != nil {
			dbg(err.Error())
		}

		// dataType = response.Header.Get("content-type")

		data, err = readAll(response.Body, length)
		if err != nil {
			return err
		}

		if length > 0 && len(data) != cap(data) {
			dbg("wrong Content-Length value")
		}
		return nil
	})
	return data, err
}

// same as io.ReadAll, but won't allocate new memory if size
// is known beforehand, if size is off by few bytes, will
// allocate a lot more than actually needed, since it tries
// to fit everything in allocated space first, and allocates
// new space only if read fails and EOF is not reached,
// Content-Size should not be wrong, but as google search
// results say, there were instances of it being wrong,
// won't work with files (since they aren't null-terminated?)
func readAll(src io.Reader, size int) ([]byte, error) {
	var (
		newPos, n int
		err       error
	)
	allocSize := chunkSize
	if size > 0 {
		allocSize = size
	}
	buf := make([]byte, 0, allocSize)

	for err == nil {
		// should not happen
		if len(buf) == cap(buf) && n == 0 {
			buf = append(buf, 0)[:len(buf)]
		}

		newPos = len(buf) + chunkSize
		if newPos > cap(buf) {
			newPos = cap(buf)
		}

		n, err = src.Read(buf[len(buf):newPos])
		buf = buf[:len(buf)+n]
	}

	if err == io.EOF {
		err = nil
	}

	return buf, err

}

// ### remove
/*
// TODO: cancel response body readings for unfinished tracks
// will solve a lot of problems
// TODO: maybe it is a good idea to check domain everytime, just in case?
func download(link string, mobile, checkDomain bool) (io.ReadCloser, string, error) {
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
	// ~~64~~128k is not enough for all pages apparently
	// failed on a page with ~~43~~ 105 tracks
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

/*func processTagPage(args *arguments, message chan<- interface{}) {
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

	url := sbuilder.String()
	message <- url
	reader, _, err := download(url, false, true)
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
		message <- "empty search results must be sent, but alas (should they?)"
		message <- err
		return
	}
	// pretend that we are Chrome on Win10
	request.Header.Set("User-Agent", userAgent)

	response, err := client.Do(request)
	if err != nil {
		// window.sendEvent(newAdditionalTagSearch(nil))
		message <- "empty search results must be sent, but alas (should they?)"
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
*/
