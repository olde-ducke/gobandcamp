package main

import (
	"bufio"
	"bytes"
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
	window.handlePlayerEvent("fetching...")
	request, err := http.NewRequest("GET", link, nil)
	if err != nil {
		window.handlePlayerEvent(err.Error())
		return nil
	}
	// pretend that we are Chrome on Win10
	request.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/93.0.4577.82 Safari/537.36")
	// set mobile view, html weights a bit less than desktop version
	if mobile {
		request.Header.Set("Cookie", "mvp=p")
	}

	window.handlePlayerEvent("downloading...")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		window.handlePlayerEvent(err.Error())
		return nil
	}
	// not all artists are hosted on bandname.bandcamp.com,
	// deal with aliases by reading canonical names from response
	if checkDomain {
		if !strings.Contains(response.Header.Get("Link"),
			"bandcamp.com") {
			window.handlePlayerEvent("Response came not from bandcamp.com")
			response.Body.Close()
			return nil
		}
	}
	return response.Body
}

func processMediaPage(link string, mobile bool) {
	reader := download(link, mobile, true)
	if reader == nil {
		return
	}
	defer reader.Close()

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
				window.handlePlayerEvent("unexpected page format")
				return
			}
			replacer := strings.NewReplacer(`&quot;`, `"`, `&amp;`, `&`)
			mediaDataJSON = replacer.Replace(line[start:end])
		}
	}

	//_, err = io.Copy(file, response.Body)
	//checkFatalError(err)
	if metaDataJSON != "" && mediaDataJSON != "" {
		if !album {
			window.handlePlayerEvent("found track data (not implemented)")
		} else {
			window.handlePlayerEvent("found album data")
			window.playerM.album, window.playerM.media =
				parseAlbumJSON(metaDataJSON, mediaDataJSON)
			player.totalTracks = window.playerM.album.Tracks.NumberOfItems
			window.handlePlayerEvent(eventPlay(0))
		}
	} else {
		window.handlePlayerEvent("unexpected page format")
	}
}

func downloadMedia(link string) {
	track := player.currentTrack
	if cache[track] != nil {
		window.handlePlayerEvent(eventTrackDownloader(track))
		window.handlePlayerEvent("playing from cache")
		return
	}
	reader := download(link, false, false)
	if reader == nil {
		return
	}
	defer reader.Close()
	body, err := io.ReadAll(reader)
	if err != nil {
		window.handlePlayerEvent(err.Error())
		return
	}
	cache[track] = body
	window.handlePlayerEvent(eventTrackDownloader(track))
	window.handlePlayerEvent("download finished")

}

/*func getAlbumPage(link string) (jsonString string, err error) {
	request, err := createNewRequest(link, false)
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
}*/

// TODO: methods below don't need to be methods, message can be formed on caller side,
// they really need only media links and channel where signal could be sent
/*func (player *playback) downloadCover() {
	request, err := createNewRequest(player.albumList.Image, false)
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
	request, err = createNewRequest(httpLink, false)
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
			request, err := createNewRequest(value.Value.(string), false)
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
}*/

// TODO: bandcamp tag request and parser
