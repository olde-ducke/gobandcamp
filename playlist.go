package main

import (
	"errors"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Mode playback mode: normal, repeat, repeat one or random.
type Mode int

const (
	normal Mode = iota
	repeat
	repeatOne
	random
)

var modes = [4]string{"normal", "repeat", "repeat one", "random"}
var Open = func(string) error { return nil }

func (mode Mode) String() string {
	return modes[mode]
}

type playbackMode interface {
	next(int, int) int
	prev(int, int) int
	switchTrack(int, int, Player) int
	get() Mode
}

type repeatMode struct {
	mode Mode
}

func (m *repeatMode) next(current, total int) int {
	return (current + 1) % total
}

func (m *repeatMode) prev(current, total int) int {
	return (total + current - 1) % total
}

func (m *repeatMode) switchTrack(current, total int, p Player) int {
	return m.next(current, total)
}

func (m *repeatMode) get() Mode {
	return m.mode
}

type defaultMode struct {
	repeatMode
}

func (m *defaultMode) switchTrack(current, total int, p Player) int {
	if current == total-1 {
		p.Reload()
		return current
	}
	return m.next(current, total)
}

type randomMode struct {
	repeatMode
}

// FIXME: bad way of doing random track
func (m *randomMode) next(current, total int) int {
	if total < 2 {
		return current
	}

	rand.Seed(time.Now().UnixNano())
	// never play same track again if in random mode
	previous := current
	for current == previous {
		current = rand.Intn(total)
	}
	return current
}

func (m *randomMode) prev(current, total int) int {
	return m.next(current, total)
}

// PlaylistItem is a metadata of a media item.
type PlaylistItem struct {
	Unreleased  bool
	Streaming   int
	Path        string
	Title       string
	Artist      string
	Date        time.Time
	Tags        string
	Album       string
	AlbumURL    string
	TrackNum    int
	TrackArtist string
	TrackURL    string
	ArtPath     string
	TotalTracks int
	Duration    float64
}

// Playlist simple playlist manager.
type Playlist struct {
	sync.RWMutex
	dbg     func(string)
	player  Player
	current int
	data    []PlaylistItem
	size    int
	mode    playbackMode
}

// Prev switches to previous track.
// Resets current track to beginning if position is over 5s.
func (p *Playlist) Prev() {
	if p.IsEmpty() {
		return
	}

	pos := p.player.GetTime().Seconds()
	p.player.Stop()
	p.player.SetStatus(skipBWD)

	// reset position rather than switching back
	// if position is less than 5 seconds
	if pos > 5 {
		err := p.player.Reload()
		if err != nil {
			p.dbg(err.Error())
		}
		return
	}

	nextTrack := p.mode.prev(p.current, p.GetTotalTracks())
	p.SetTrack(nextTrack)
	err := Open(p.GetCurrentItem().Path)
	if err != nil {
		p.dbg(err.Error())
	}
}

// Next switches to next track.
func (p *Playlist) Next() {
	if p.IsEmpty() {
		return
	}
	p.player.Stop()
	p.player.SetStatus(skipFWD)
	nextTrack := p.mode.next(p.current, p.GetTotalTracks())
	p.SetTrack(nextTrack)
	err := Open(p.GetCurrentItem().Path)
	if err != nil {
		p.dbg(err.Error())
	}
}

// Switch is called by player at the end of playback.
func (p *Playlist) Switch() {
	p.player.Stop()
	p.player.SetStatus(skipFWD)
	nextTrack := p.mode.switchTrack(p.current, p.GetTotalTracks(), p.player)
	p.SetTrack(nextTrack)
	err := Open(p.GetCurrentItem().Path)
	if err != nil {
		p.dbg(err.Error())
	}
	p.dbg("NOT IMPLEMENTED: track switching")
}

// Clear deletes all playlist data.
func (p *Playlist) Clear() {
	p.data = make([]PlaylistItem, 0, p.size)
}

// GetMode returns current playback mode.
func (p *Playlist) GetMode() Mode {
	return p.mode.get()
}

// SetMode sets playback mode to given mode.
// Invalid values will be ignored, and mode
// will be set to 'normal'.
func (p *Playlist) SetMode(mode Mode) {
	switch mode {
	case normal:
		p.mode = &defaultMode{repeatMode{mode: mode}}
	case repeat, repeatOne:
		p.mode = &repeatMode{mode: mode}
	case random:
		p.mode = &randomMode{repeatMode{mode: mode}}
	default:
		p.dbg("invalid playback mode, setting to normal")
		p.mode = &defaultMode{repeatMode{mode: normal}}
	}
}

// NextMode switches to next playback mode.
func (p *Playlist) NextMode() {
	p.SetMode((p.mode.get() + 1) % 4)
}

// GetCurrentTrack returns current playing tracks
// number in playlist.
func (p *Playlist) GetCurrentTrack() int {
	if p.IsEmpty() {
		return 0
	}
	return p.current + 1
}

// GetTotalTracks returns total number of tracks in playlist.
func (p *Playlist) GetTotalTracks() int {
	return len(p.data)
}

// SetTrack sets playlist to play given track number.
func (p *Playlist) SetTrack(track int) {
	p.dbg(strconv.Itoa(track))
	p.current = track
}

// Enqueue appends items to the end of playlist.
// TODO: remove any mentions of outside objects
func (p *Playlist) Enqueue(items []item) error {
	var err error
	p.Lock()
	defer p.Unlock()
	for _, i := range items {
		if !i.hasAudio {
			p.dbg(i.title + " has no available audio")
			if err == nil {
				err = errors.New("some items skipped, because they have no available audio ")
			}
			continue
		}
		for _, t := range i.tracks {
			if len(p.data) == cap(p.data) {
				return errors.New("can't have more than " +
					strconv.Itoa(p.size) + " tracks")
			}
			p.data = append(p.data, PlaylistItem{
				Unreleased:  t.unreleasedTrack,
				Streaming:   t.streaming,
				Path:        t.mp3128,
				Title:       t.title,
				Artist:      i.artist,
				Date:        i.albumReleaseDate,
				Tags:        strings.Join(i.tags, " "),
				Album:       i.title,
				AlbumURL:    i.url, // FIXME: build url from art id
				TrackNum:    t.trackNumber,
				TrackArtist: t.artist,
				TrackURL:    t.url,
				ArtPath:     i.artURL,
				TotalTracks: i.totalTracks,
				Duration:    t.duration,
			})
		}
	}
	return err
}

// Add first clears playlist then adds new items.
func (p *Playlist) Add(items []item) error {
	p.Clear()
	if err := p.Enqueue(items); err != nil {
		return err
	}

	if p.IsEmpty() {
		return errors.New("no streamable media was found")
	}

	p.SetTrack(0)
	return nil
}

// GetCurrentItem returns current item metadata
func (p *Playlist) GetCurrentItem() *PlaylistItem {
	if p.IsEmpty() {
		return nil
	}
	return &p.data[p.current]
}

// IsEmpty reports if playlist is empty or not.
func (p *Playlist) IsEmpty() bool {
	return len(p.data) == 0
}

// NewPlaylist TBD
func NewPlaylist(player Player, dbg func(string)) *Playlist {
	p := &Playlist{
		dbg:    dbg,
		player: player,
		size:   1024,
		mode:   &defaultMode{repeatMode{mode: normal}},
	}
	player.SetCallback(p.Switch)
	p.data = make([]PlaylistItem, 0, p.size)
	return p
}
