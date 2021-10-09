package main

import (
	"fmt"
	"image"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/qeesung/image2ascii/convert"
)

type screenRect struct {
	minX, minY, maxX, maxY int
}

type screenDrawer struct {
	textArea  screenRect
	screen    tcell.Screen
	style     tcell.Style
	hMargin   int // horizontal margin
	vMargin   int
	artMode   int
	lightMode bool
	bgColor   tcell.Color
	fgColor   tcell.Color
	altColor  tcell.Color
}

func (drawer *screenDrawer) reDrawMetaData(player *playback) {
	drawer.screen.Fill(' ', drawer.style)
	drawer.textArea.minX = drawer.redrawArt(player) + drawer.hMargin*2
	drawer.textArea.maxX, drawer.textArea.maxY = drawer.screen.Size()
	drawer.updateTextData(player)
	drawer.screen.Sync()
}

// draw album art
func (drawer *screenDrawer) redrawArt(player *playback) int {
	art := convert.NewImageConverter().Image2CharPixelMatrix(
		player.albumList.AlbumArt, &convert.DefaultOptions)

	x, y := drawer.hMargin, 0
	for _, pixelY := range art {
		for _, pixelX := range pixelY {
			switch drawer.artMode {
			// fill all cells with colourful spaces
			case 0:
				drawer.screen.SetContent(x, y, ' ', nil,
					drawer.style.Background(tcell.NewRGBColor(int32(pixelX.R),
						int32(pixelX.G), int32(pixelX.B))))
			// foreground is darker shade, background lighter shade
			case 1:
				style := tcell.StyleDefault.Background(
					tcell.NewRGBColor(int32(pixelX.R),
						int32(pixelX.G), int32(pixelX.B))).Foreground(
					tcell.NewRGBColor(int32(pixelX.R)/2,
						int32(pixelX.G)/2, int32(pixelX.B)/2))
				drawer.screen.SetContent(x, y, rune(pixelX.Char), nil, style)
			// opposite
			case 2:
				style := tcell.StyleDefault.Background(
					tcell.NewRGBColor(int32(pixelX.R)/2,
						int32(pixelX.G)/2, int32(pixelX.B)/2)).Foreground(
					tcell.NewRGBColor(int32(pixelX.R),
						int32(pixelX.G), int32(pixelX.B)))
				drawer.screen.SetContent(x, y, rune(pixelX.Char), nil, style)
			// draws art with colourful symbols on background color
			case 3:
				drawer.screen.SetContent(x, y, rune(pixelX.Char), nil,
					drawer.style.Foreground(tcell.NewRGBColor(int32(pixelX.R),
						int32(pixelX.G), int32(pixelX.B))))
			// draw colourful background with dark theme symbols
			case 4:
				style := tcell.StyleDefault.Foreground(drawer.bgColor)
				drawer.screen.SetContent(x, y, rune(pixelX.Char), nil,
					style.Background(tcell.NewRGBColor(int32(pixelX.R),
						int32(pixelX.G), int32(pixelX.B))))
			// same, but with light theme symbols
			case 5:
				style := tcell.StyleDefault.Foreground(drawer.fgColor)
				drawer.screen.SetContent(x, y, rune(pixelX.Char), nil,
					style.Background(tcell.NewRGBColor(int32(pixelX.R),
						int32(pixelX.G), int32(pixelX.B))))
			}
			x++
		}
		x = drawer.hMargin
		y++
	}
	return len(art[0])
}

// TODO: playlist and lyrics display
func (drawer *screenDrawer) updateTextData(player *playback) {
	// TODO: replace this mess with tcell/views?

	messagePos := drawer.textArea.maxY - 1 - drawer.vMargin
	for i := 1; i <= messagePos; i++ {
		drawer.clearString(drawer.textArea.minX, i)
	}

	if messagePos > 10 {
		drawer.drawString(drawer.textArea.minX, messagePos,
			player.latestMessage, true)
	}

	var sbuilder strings.Builder
	if len(player.albumList.Tracks.ItemListElement) == 0 {
		drawer.screen.Show()
		return
	}
	item := player.albumList.Tracks.ItemListElement[player.currentTrack]

	drawer.drawString(drawer.textArea.minX, drawer.vMargin, player.albumList.Name, false)
	drawer.drawString(drawer.textArea.minX, drawer.vMargin+1, "by", true)
	drawer.drawString(drawer.textArea.minX+3, drawer.vMargin+1,
		player.albumList.ByArtist["name"].(string), false)

	fmt.Fprintf(&sbuilder, "released %s", player.albumList.DatePublished[:11])
	drawer.drawString(drawer.textArea.minX, drawer.vMargin+2, sbuilder.String(), false)
	sbuilder.Reset()

	for i, value := range player.albumList.Tags {
		if i == 0 {
			fmt.Fprint(&sbuilder, value)
		} else {
			fmt.Fprint(&sbuilder, " ", value)
		}
	}
	drawer.drawString(drawer.textArea.minX, drawer.vMargin+3, sbuilder.String(), true)
	sbuilder.Reset()

	fmt.Fprintf(&sbuilder, "%2s %2d/%d - %s", player.status.String(),
		item.Position, player.albumList.Tracks.NumberOfItems,
		item.TrackInfo.Name)
	drawer.drawString(drawer.textArea.minX, drawer.vMargin+5, sbuilder.String(), false)
	sbuilder.Reset()

	var seconds float64
	for _, value := range item.TrackInfo.AdditionalProperty {
		if value.Name == "duration_secs" {
			seconds = value.Value.(float64)
		}
	}

	if seconds != 0 {
		var repeats int = int(100*player.currentPos/
			time.Duration(seconds*1_000_000_000)) *
			(drawer.textArea.maxX - drawer.textArea.minX - drawer.hMargin) /
			100

		drawer.drawString(drawer.textArea.minX, drawer.vMargin+6,
			strings.Repeat("\u25b1", repeats), false)
		sbuilder.Reset()
	}

	fmt.Fprintf(&sbuilder, "%s/%s", player.currentPos,
		time.Duration(seconds*1_000_000_000).Round(time.Second))
	drawer.drawString(drawer.textArea.minX, drawer.vMargin+7, sbuilder.String(), false)
	sbuilder.Reset()

	if player.muted {
		fmt.Fprintf(&sbuilder, "volume mute")
	} else {
		fmt.Fprintf(&sbuilder, "volume %3.0f ", (100 + player.volume*10))
	}
	fmt.Fprintf(&sbuilder, " mode %s", player.playbackMode.String())
	drawer.drawString(drawer.textArea.minX, drawer.vMargin+8, sbuilder.String(), false)
	sbuilder.Reset()

	drawer.screen.Show()
}

func (drawer *screenDrawer) drawString(x, y int, str string, altStyle bool) {
	style := drawer.style
	if altStyle {
		style = drawer.style.Foreground(drawer.altColor)
	}
	for _, r := range str {
		if x == drawer.textArea.maxX-drawer.hMargin {
			x -= 3
			for i := x; i < drawer.textArea.maxX-drawer.hMargin; i++ {
				drawer.screen.SetContent(i, y, '.', nil, style)
			}
			return
		}
		drawer.screen.SetContent(x, y, r, nil, style)
		x++
	}
}

func (drawer *screenDrawer) clearString(x, y int) {
	for i := x; i < drawer.textArea.maxX-drawer.hMargin; i++ {
		drawer.screen.SetContent(i, y, ' ', nil, drawer.style)
	}
}

func (drawer *screenDrawer) initScreen(player *playback) {
	var err error
	drawer.screen, err = tcell.NewScreen()
	reportError(err)
	err = drawer.screen.Init()
	reportError(err)
	if drawer.lightMode {
		drawer.style = tcell.StyleDefault.
			Foreground(drawer.bgColor).
			Background(drawer.fgColor)
	} else {
		drawer.style = tcell.StyleDefault.
			Foreground(drawer.fgColor).
			Background(drawer.bgColor)
	}
	drawer.screen.Fill(' ', drawer.style)
	drawer.reDrawMetaData(player)
	drawer.hMargin = 3
	drawer.vMargin = 1
}

// TODO: actual placeholder something with gopher?
func getPlaceholderImage() image.Image {
	return image.NewRGBA(image.Rect(0, 0, 1, 1))
}
