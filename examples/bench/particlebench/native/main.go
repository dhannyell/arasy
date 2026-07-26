//go:build windows && amd64

package main

import (
	"log/slog"
	"os"

	"github.com/dhannyell/arasy/examples/bench/particlebench"
	"github.com/dhannyell/arasy/platform/native"
)

func main() {
	game, err := particlebench.NewGame()
	if err != nil {
		panic(err)
	}
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	if err := native.Run(game, native.Options{
		Title: "Particle Benchmark", Width: 1980, Height: 1080, Log: log,
		Metrics: false,
		FrameMetrics: func(m native.FrameMetrics) {
			game.ObserveFrame(
				m.FPS,
				m.FrameTime,
				m.P99FrameTime,
				m.InputToSubmit,
				m.InputToSubmitSet,
				m.HeapInUse,
				m.HeapSys,
				m.HeapSet,
				m.GPUBufferBytes,
				m.GPUBufferSet,
				0,
				0,
				0,
				false,
			)
		},
		VSync: native.VSyncOff,
	}); err != nil {
		log.Error("native host error", "err", err)
		os.Exit(1)
	}
}
