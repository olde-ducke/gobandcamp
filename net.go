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

// Debugf package level debug printing function.
var Debugf = func(string, ...any) {}

var errOrigin = errors.New("response came not from bandcamp.com")
var errUnexpected = errors.New("unexpected page format")
var client = http.Client{Timeout: 120 * time.Second}

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

func processmediapage(ctx context.Context, link string, infof func(string, ...any)) ([]item, error) {
	Debugf(link)
	infof("fetching")
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
		infof(response.Status)

		// check canonical name in response header
		// must be artist.bandcamp.com
		if !strings.Contains(response.Header.Get("link"), "bandcamp.com") {
			return errOrigin
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
			Debugf("failed to parse page type")
			return errUnexpected
		}

		switch itemType {
		case "album":
			infof("found album data")

		case "song":
			infof("found track data")

		case "band":
			infof("found discography")
			// TODO: finish
			node, ok := getNodeWithAttr(doc, &html.Attribute{
				Key: "id",
				Val: "music-grid",
			}, "ol")
			if !ok {
				return errUnexpected
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
						res, err := processmediapage(ctx, u, func(string, ...any) {})
						if err != nil {
							Debugf(err.Error())
						} else if len(res) > 1 {
							Debugf("unexpected results: more than 1 album")
						} else if len(res) > 0 {
							items[n] = res[0]
							Debugf("done with " + u)
						}
					}(i, url.String())
				}
				wg.Wait()

				if len(items) > 0 {
					return nil
				}
			}
			return errUnexpected

		default:
			return errUnexpected
		}

		mediadata, ok := getValWithAttr(doc, &html.Attribute{
			Key: "type",
			Val: "text/javascript",
		}, "script", "data-tralbum")
		if !ok {
			Debugf("failed to parse mediadata")
			return errUnexpected
		}

		metadata, ok := getTextWithAttr(doc, &html.Attribute{
			Key: "type",
			Val: "application/ld+json",
		}, "script")
		if !ok {
			return errUnexpected
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
func downloadmedia(ctx context.Context, link string, infof func(string, ...any)) ([]byte, error) {
	Debugf(link)
	infof("fetching")
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
		infof(response.Status)

		lengthValue := response.Header.Get("Content-Length")
		length, err := strconv.Atoi(lengthValue)
		if err != nil {
			Debugf(err.Error())
		}

		// dataType = response.Header.Get("content-type")

		progress := 0
		done := make(chan struct{})
		go func() {
		loop:
			for {
				select {
				case <-done:
					break loop
				default:
					infof("downloading... %d%%", 100*progress/length)
					time.Sleep(300 * time.Millisecond)
				}
			}
			Debugf("update routine ended")
		}()
		defer close(done)

		data, err = readAll(response.Body, length, &progress)
		if err != nil {
			return err
		}

		if length > 0 && len(data) != cap(data) {
			Debugf("wrong Content-Length value")
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
func readAll(src io.Reader, size int, p *int) ([]byte, error) {
	var (
		newPos, n int
		err       error
	)
	allocSize := chunkSize
	if size > 0 {
		allocSize = size
	}
	buf := make([]byte, 0, allocSize)

	for {
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
		*p += n

		if errors.Is(err, io.EOF) {
			break
		}

		if err != nil {
			return nil, err
		}
	}

	return buf, nil

}

/*
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
