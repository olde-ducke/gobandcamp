package main

import (
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
	return &bytesReadSeekCloser{bytes.NewReader(cachedResponses[index])}
}

func createNewRequest(link string) (*http.Request, error) {
	request, err := http.NewRequest("GET", link, nil)
	if err != nil {
		return nil, err
	}
	// pretend that we are Chrome on Win10
	request.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/93.0.4577.82 Safari/537.36")
	// set mobile view, html weights a bit less than desktop version
	// TODO: bandcamp tag search is all sorts of not okay on mobile
	// version
	request.Header.Set("Cookie", "mvp=p")
	return request, nil
}

func getAlbumPage(link string) (jsonString string, err error) {
	request, err := createNewRequest(link)
	if err != nil {
		return "", err
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return "", err
	}
	if response.StatusCode > 299 {
		return "", errors.New(
			fmt.Sprintf("Request failed with status code: %d\n",
				response.StatusCode),
		)
	}
	body, err := io.ReadAll(response.Body)
	// seems reasonable to crash, if we can't close reader
	checkFatalError(response.Body.Close())
	if err != nil {
		fmt.Println("Error:", err.Error())
		return "", err
	}

	// not all artists are hosted on bandname.bandcamp.com,
	// deal with aliases by reading canonical names from response
	if !strings.Contains(response.Header.Get("Link"),
		"bandcamp.com") {
		return "", errors.New("Response came not from bandcamp.com")
	}

	reader := bytes.NewBuffer(body)

	for {
		jsonString, err = reader.ReadString('\n')
		if err != nil {
			fmt.Println(err.Error())
			return "", err
		}
		if strings.Contains(jsonString, "application/ld+json") {
			jsonString, err = reader.ReadString('\n')
			if err != nil {
				return "", err
			}
			break
		}
	}
	return jsonString, nil
}

// TODO: methods below don't need to be methods, message can be formed on caller side,
// they really need only media links and channel where signal could be sent
func (player *playback) downloadCover() {
	request, err := createNewRequest(player.albumList.Image)
	if err != nil {
		player.albumList.AlbumArt = getPlaceholderImage()
		player.event <- err.Error()
		return
	}
	// images requests over https fail with EOF error
	// for me lately, not even official app can download
	// covers/avatars/etc, request doesn't fail over http
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		player.latestMessage = fmt.Sprint("https://... image request failed with error:",
			err, "trying http://...")
	}
	httpLink := strings.Replace(player.albumList.Image, "https://",
		"http://", 1)
	request, err = createNewRequest(httpLink)
	if err != nil {
		player.albumList.AlbumArt = getPlaceholderImage()
		player.event <- err.Error()
		return
	}
	response, err = http.DefaultClient.Do(request)
	if err != nil {
		player.albumList.AlbumArt = getPlaceholderImage()
		player.event <- err.Error()
		return
	}
	defer response.Body.Close()

	switch response.Header.Get("Content-Type") {
	case "image/jpeg":
		player.albumList.AlbumArt, err = jpeg.Decode(response.Body)
		if err != nil {
			player.albumList.AlbumArt = getPlaceholderImage()
			player.event <- err.Error()
			return
		}
	/*
		// TODO: delete later: there seem to be only jpeg covers
		// in case if planned placeholder image will be in png, can actually be
		// restored
		case "image/png":
			player.albumList.AlbumArt, _ = png.Decode(response.Body)
	*/
	default:
		player.albumList.AlbumArt = getPlaceholderImage()
		player.event <- "Album cover is not jpeg image"
	}
	player.event <- "Album cover downloaded"
}

func (player *playback) getNewTrack(trackNumber int) {
	if len(player.albumList.Tracks.ItemListElement) == 0 {
		player.latestMessage = "No album data was found"
		return
	}
	item := player.albumList.Tracks.ItemListElement[trackNumber]
	filename := fmt.Sprint(player.albumList.ByArtist["name"], " - ", item.TrackInfo.Name, ".mp3")

	if _, ok := cachedResponses[trackNumber]; ok {
		player.latestMessage = fmt.Sprint(filename, " - Cached")
		player.event <- trackNumber
		return
	}

	player.latestMessage = fmt.Sprint(filename, " - Fetching...")
	for _, value := range item.TrackInfo.AdditionalProperty {
		// not all tracks are available for streaming,
		// there is a `streaming` field in JSON
		// but tracks that haven't been published yet
		// (preorder items with some tracks available
		// for streaming) don't have it at all
		if value.Name == "file_mp3-128" {
			request, err := createNewRequest(value.Value.(string))
			if err != nil {
				player.latestMessage = fmt.Sprintf(
					"Request cannot be made: %s\n", err.Error())
				return
			}
			response, err := http.DefaultClient.Do(request)
			if err != nil {
				player.latestMessage = fmt.Sprintf(
					"Request failed: %s\n", err.Error())
				return
			}
			if response.StatusCode > 299 {
				player.latestMessage = fmt.Sprintf(
					"Request failed with status code: %d\n", response.StatusCode)
				return
			}
			player.latestMessage = fmt.Sprint(filename, " - ", response.Status, " Downloading...")

			bodyBytes, err := io.ReadAll(response.Body)
			defer checkFatalError(response.Body.Close())
			if err != nil {
				player.latestMessage = err.Error()
				return
			}

			cachedResponses[trackNumber] = bodyBytes
			player.latestMessage = fmt.Sprint(filename, " - Done")
			player.event <- trackNumber
			return
		}
	}
	player.latestMessage = "Track is currently not available for streaming"
}

// TODO: bandcamp tag request and parser
