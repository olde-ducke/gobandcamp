package main

import (
	"fmt"
	"math/rand"
	"sync"
	"time"
)

// Mode playback mode: normal, repeat, repeat one or random.
type Mode int

// TODO: random repeat is missing
const (
	normal Mode = iota
	repeat
	repeatOne
	random
)

var (
	modeNames = [4]string{"normal", "repeat", "repeat one", "random"}
	Open      = func(string) error { return nil }

	modes [4]playbackMode
)

func (mode Mode) String() string {
	return modeNames[mode]
}

type playbackMode interface {
	nextTrack(int, int) int
	prevTrack(int, int) int
	switchTrack(int, int, Player) int
	get() Mode
}

type repeatMode struct {
	mode Mode
}

func (m *repeatMode) nextTrack(current, total int) int {
	return (current + 1) % total
}

func (m *repeatMode) prevTrack(current, total int) int {
	return (total + current - 1) % total
}

func (m *repeatMode) switchTrack(current, total int, p Player) int {
	return m.nextTrack(current, total)
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
		p.Stop()
		return current
	}
	return m.nextTrack(current, total)
}

type repeatOneMode struct {
	repeatMode
}

func (m *repeatOneMode) switchTrack(current, total int, p Player) int {
	p.Reload()
	return current
}

type randomMode struct {
	repeatMode
}

// FIXME: bad way of doing random track
func (m *randomMode) nextTrack(current, total int) int {
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

func (m *randomMode) prevTrack(current, total int) int {
	return m.nextTrack(current, total)
}

func (m *randomMode) switchTrack(current, total int, p Player) int {
	return m.nextTrack(current, total)
}

// PlaylistItem is a metadata of a media item.
type PlaylistItem struct {
	Unreleased  bool
	Streaming   bool
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

	nextTrack := p.mode.prevTrack(p.current, p.GetTotalTracks())
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
	nextTrack := p.mode.nextTrack(p.current, p.GetTotalTracks())
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
}

// Clear deletes all playlist data.
func (p *Playlist) Clear() {
	p.Lock()
	p.data = make([]PlaylistItem, 0, p.size)
	p.Unlock()
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
	case normal, repeat, repeatOne, random:
		p.mode = modes[mode]
	default:
		p.dbg("invalid playback mode, setting to normal")
		p.mode = modes[normal]
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
	p.current = track
}

// Enqueue appends items to the end of playlist.
// TODO: remove any mentions of outside objects
func (p *Playlist) Enqueue(items []PlaylistItem) error {
	p.Lock()
	defer p.Unlock()
	for _, i := range items {
		if len(p.data) == cap(p.data) {
			return fmt.Errorf("can't have more than %d tracks", p.size)
		}
		p.data = append(p.data, i)
	}

	return nil
}

// Add first clears playlist then adds new items.
func (p *Playlist) Add(items []PlaylistItem) error {
	p.player.Stop()
	p.player.SetStatus(skipFWD)
	p.Clear()

	if len(items) > p.size {
		return p.Enqueue(items)
	}

	p.Lock()
	p.data = items
	p.Unlock()

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
func NewPlaylist(player Player, size int, dbg func(string)) *Playlist {
	modes[normal] = &defaultMode{repeatMode{mode: normal}}
	modes[repeat] = &repeatMode{mode: repeat}
	modes[repeatOne] = &repeatOneMode{repeatMode{mode: repeatOne}}
	modes[random] = &randomMode{repeatMode{mode: random}}

	p := &Playlist{
		dbg:    dbg,
		player: player,
		size:   size,
		mode:   modes[normal],
	}
	player.SetCallback(p.Switch)
	p.data = make([]PlaylistItem, 0, p.size)
	return p
}
