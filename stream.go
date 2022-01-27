package main

import (
	"bytes"

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
	ctrl := &beep.Ctrl{Streamer: streamer}
	resampler := beep.Resample(3, sampleRate, defaultSampleRate, ctrl)
	volume := &effects.Volume{Streamer: resampler, Base: 2, Volume: playerVolume, Silent: muted}
	return &mediaStream{sampleRate, streamer, ctrl, resampler, volume}
}

// hacky solution for beep requirments of readcloser
type bytesReadSeekCloser struct {
	*bytes.Reader
}

func (c bytesReadSeekCloser) Close() error {
	return nil
}

func wrapInRSC(key string) *bytesReadSeekCloser {
	// TODO: remove
	value, ok := cache.get(key)
	if !ok {
		return &bytesReadSeekCloser{bytes.NewReader([]byte{0})}
	}
	// and accept bytes as input, whatever starts playback should give
	// player bytes, beep player would wrap them in this nonsense
	return &bytesReadSeekCloser{bytes.NewReader(value)}
}
