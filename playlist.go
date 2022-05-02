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

func (mode Mode) String() string {
	return modes[mode]
}

type playbackMode interface {
	next(int, int) int
	prev(int, int) int
	get() Mode
}

type defaultMode struct {
	mode Mode
}

func (m *defaultMode) next(current, total int) int {
	return (current + 1) % total
}

func (m *defaultMode) prev(current, total int) int {
	return (total + current - 1) % total
}

func (m *defaultMode) get() Mode {
	return m.mode
}

type randomMode struct {
	defaultMode
}

// FIXME: bad way of doing random track
func (rm *randomMode) next(current, total int) int {
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

func (rm *randomMode) prev(current, total int) int {
	return rm.next(current, total)
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

	// reset position rather than switching back
	// if position is less than 5 seconds
	if p.player.GetTime().Seconds() > 5 {
		err := p.player.SeekAbsolute(0)
		if err != nil {
			p.dbg(err.Error())
		}
		return
	}

	nextTrack := p.mode.prev(p.current, p.GetTotalTracks())
	p.SetTrack(nextTrack)
}

// Next switches to next track.
func (p *Playlist) Next() {
	if p.IsEmpty() {
		return
	}
	nextTrack := p.mode.next(p.current, p.GetTotalTracks())
	p.SetTrack(nextTrack)
}

// Start TBD.
func (p *Playlist) Start() {
	p.dbg("playlist: NOT IMPLEMENTED next")
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
	case repeat, repeatOne, normal:
		p.mode = &defaultMode{mode: mode}
	case random:
		p.mode = &randomMode{defaultMode{mode: mode}}
	default:
		p.mode = &defaultMode{mode: normal}
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
	if track == p.current {
		p.player.Reload()
		return
	}

	p.current = track
}

// Enqueue appends items to the end of playlist.
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
		size:   5,
		mode:   &defaultMode{mode: normal},
	}
	p.data = make([]PlaylistItem, 0, p.size)
	return p
}

/*
func (player *beepPlayer) getPlaybackMode() string {
	return player.playbackMode.String()
}

func (player *beepPlayer) skip(forward bool) bool {
	if player.totalTracks == 0 {
		return false
	}

	if player.playbackMode == random {
		player.nextTrack()
		return true
	}

	player.dbg("skip track")
	player.stop()
	player.clearStream()

	if forward {
		player.currentTrack = (player.currentTrack + 1) %
			player.totalTracks
		player.status = skipFWD
	} else {
		player.currentTrack = (player.totalTracks +
			player.currentTrack - 1) %
			player.totalTracks
		player.status = skipBWD
	}

	return true
}

func (player *beepPlayer) nextMode() {
	player.playbackMode = (player.playbackMode + 1) % 4
}

func (player *beepPlayer) nextTrack() {
	player.dbg("next track")
	switch player.playbackMode {

	case random:
		var previousTrack int

		if player.totalTracks > 1 {
			rand.Seed(time.Now().UnixNano())
			previousTrack = player.currentTrack
			// never play same track again if random
			for player.currentTrack == previousTrack {
				player.currentTrack = rand.Intn(player.totalTracks)
			}
		}
		player.stop()

		if player.currentTrack >= previousTrack {
			player.status = skipFWD
		} else {
			player.status = skipBWD
		}

		player.clearStream()

	// beep does have loop one, but stream should be set
	// up to loop from the very start to play indefinetly,
	// which is not ideal
	case repeatOne:
		// doesn't work without position reset?
		player.resetPosition()
		player.restart()

	case repeat:
		player.skip(true)

	case normal:
		if player.currentTrack == player.totalTracks-1 {
			// prepare new stream for playback again
			// and immediately stop it
			player.restart()
			player.stop()
			return
		}
		player.skip(true)
	}
}

func (player *beepPlayer) setTrack(track int) bool {
	if track >= player.totalTracks || track < 0 || track == player.currentTrack {
		return false
	}

	player.stop()
	player.clearStream()
	if track >= player.currentTrack {
		player.status = skipFWD
	} else {
		player.status = skipBWD
	}
	player.currentTrack = track
	return true
}

// TODO: move to playlist
type playbackMode int

 const (
	normal playbackMode = iota
	repeat
	repeatOne
	random
)

var modes = [4]string{"normal", "repeat", "repeat one", "random"}

func (mode playbackMode) String() string {
	return modes[mode]
}*/
