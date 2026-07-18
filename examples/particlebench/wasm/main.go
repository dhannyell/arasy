//go:build js && wasm

package main

import (
	"log/slog"

	"github.com/dhannyell/frameforge/examples/particlebench"
	"github.com/dhannyell/frameforge/platform/wasm"
)

func main() {
	game, err := particlebench.NewGame()
	if err != nil {
		panic(err)
	}
	log := wasm.NewConsoleLogger(slog.LevelInfo)
	if err := wasm.Run(game, wasm.Options{
		ForceCanvas2D: false,
		Log:           log,
		FrameMetrics: func(metrics wasm.FrameMetrics) {
			game.ObserveFrame(
				metrics.FPS,
				metrics.FrameTime,
				metrics.P99FrameTime,
				metrics.InputToSubmit,
				metrics.InputToSubmitSet,
				metrics.JSHeapUsed,
				metrics.JSHeapLimit,
				metrics.JSHeapSet,
				metrics.GPUBufferBytes,
				metrics.GPUBufferSet,
			)
		},
	}); err != nil {
		log.Error("wasm host failed", "err", err)
	}
}
