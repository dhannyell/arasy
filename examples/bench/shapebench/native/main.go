//go:build windows && amd64

package main

import (
	"log/slog"
	"os"

	"github.com/dhannyell/arasy/examples/bench/shapebench"
	"github.com/dhannyell/arasy/platform/native"
)

func main() {
	game := &shapebench.Game{}
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	if err := native.Run(game, native.Options{
		Title:  "Shape Benchmark",
		Width:  1980,
		Height: 1080,
		Log:    log,
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
				m.CPUTickTime,
				m.CPURenderTime,
				m.CPUTimingSet,
				m.CPUTessTime,
				m.CPUUploadTime,
				m.CPUEncodeTime,
				m.CPUBreakdownSet,
			)
		},
		VSync: native.VSyncOff,
	}); err != nil {
		log.Error("native host error", "err", err)
		os.Exit(1)
	}
}
