package player

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
	nextTrack()
	prevTrack()
	switchTrack()
	get() Mode
	set(int)
}

type repeatMode struct {
	mode Mode
	pl   *Playlist
	p    Player
}

func (m *repeatMode) nextTrack() {
	next := (m.pl.current + 1) % m.pl.GetTotalTracks()
	m.set(next)
}

func (m *repeatMode) prevTrack() {
	total := m.pl.GetTotalTracks()
	next := (total + m.pl.current - 1) % total
	m.set(next)
}

func (m *repeatMode) switchTrack() {
	m.nextTrack()
}

func (m *repeatMode) get() Mode {
	return m.mode
}

func (m *repeatMode) set(track int) {
	m.pl.SetTrack(track)
	err := Open(m.pl.GetCurrentItem().Path)
	if err != nil {
		m.pl.dbg(err.Error())
	}
}

type defaultMode struct {
	repeatMode
}

func (m *defaultMode) switchTrack() {
	if m.pl.current == m.pl.GetTotalTracks()-1 {
		m.p.Reload()
		m.p.Stop()
		return
	}
	m.nextTrack()
}

type repeatOneMode struct {
	repeatMode
}

func (m *repeatOneMode) switchTrack() {
	m.p.Reload()
}

type randomMode struct {
	repeatMode
}

// FIXME: bad way of doing random track
func (m *randomMode) nextTrack() {
	total := m.pl.GetTotalTracks()
	if total < 2 {
		m.p.Reload()
		return
	}

	rand.Seed(time.Now().UnixNano())
	// never play same track again if in random mode
	current := m.pl.current
	previous := m.pl.current
	for current == previous {
		current = rand.Intn(total)
	}
	m.set(current)
}

func (m *randomMode) prevTrack() {
	m.nextTrack()
}

func (m *randomMode) switchTrack() {
	m.nextTrack()
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

	p.mode.prevTrack()
}

// Next switches to next track.
func (p *Playlist) Next() {
	if p.IsEmpty() {
		return
	}
	p.player.Stop()
	p.player.SetStatus(skipFWD)
	p.mode.nextTrack()
}

// Switch is called by player at the end of playback.
func (p *Playlist) Switch() {
	p.player.Stop()
	p.player.SetStatus(skipFWD)
	p.mode.switchTrack()
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
	pl := &Playlist{
		dbg:    dbg,
		player: player,
		size:   size,
	}
	player.SetCallback(pl.Switch)
	pl.data = make([]PlaylistItem, 0, pl.size)

	modes[normal] = &defaultMode{repeatMode{normal, pl, player}}
	modes[repeat] = &repeatMode{repeat, pl, player}
	modes[repeatOne] = &repeatOneMode{repeatMode{repeatOne, pl, player}}
	modes[random] = &randomMode{repeatMode{random, pl, player}}

	pl.mode = modes[normal]

	return pl
}
