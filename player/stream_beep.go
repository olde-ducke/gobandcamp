package player

import (
	"io"

	"github.com/faiface/beep"
	"github.com/faiface/beep/effects"
)

type mediaStream struct {
	sampleRate beep.SampleRate
	streamer   beep.StreamSeekCloser
	ctrl       *beep.Ctrl
	resampler  *beep.Resampler
	volume     *effects.Volume
}

func newStream(sampleRate beep.SampleRate, streamer beep.StreamSeekCloser,
	playerVolume float64, muted bool) *mediaStream {
	ctrl := &beep.Ctrl{Streamer: streamer, Paused: false}
	resampler := beep.Resample(Quality, sampleRate, DefaultSampleRate, ctrl)
	volume := &effects.Volume{Streamer: resampler, Base: 2, Volume: playerVolume, Silent: muted}
	return &mediaStream{sampleRate, streamer, ctrl, resampler, volume}
}

// NopSeekCloser returns a ReadSeekCloser with a no-op Close
// method wrapping the provided Reader r.
func NopSeekCloser(r io.ReadSeeker) io.ReadSeekCloser {
	return nopSeekCloser{r}
}

type nopSeekCloser struct {
	io.ReadSeeker
}

func (nopSeekCloser) Close() error { return nil }
