//go:build windows && amd64

package main

import (
	"log/slog"
	"os"

	"github.com/dhannyell/frameforge/examples/bench/spritebench"
	"github.com/dhannyell/frameforge/platform/native"
)

func main() {
	game, err := spritebench.NewGame()
	if err != nil {
		panic(err)
	}
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	if err := native.Run(game, native.Options{Title: "Sprite Benchmark", Width: 1980, Height: 1080, Log: log, Metrics: true,
		FrameMetrics: func(m native.FrameMetrics) {
			game.ObserveFrame(m.FPS, m.FrameTime, m.P99FrameTime, m.InputToSubmit, m.InputToSubmitSet, m.HeapInUse, m.HeapSys, m.HeapSet, m.GPUBufferBytes, m.GPUBufferSet, 0, 0, 0, false, m.CPUTickTime, m.CPURenderTime, m.CPUTimingSet, m.CPUTessTime, m.CPUUploadTime, m.CPUEncodeTime, m.CPUBreakdownSet, 0, 0, 0, 0, 0, false)
		},
	}); err != nil {
		log.Error("native host error", "err", err)
		os.Exit(1)
	}
}
