package main

import (
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
