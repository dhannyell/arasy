//go:build js && wasm

package main

import (
	"log/slog"

	"github.com/dhannyell/frameforge/examples/particles"
	"github.com/dhannyell/frameforge/platform/wasm"
)

func main() {
	game, err := particles.NewGame()
	if err != nil {
		panic(err)
	}

	log := wasm.NewConsoleLogger(slog.LevelInfo)
	if err := wasm.Run(game, wasm.Options{
		ForceCanvas2D: false,
		DevTools:      true,
		Log:           log,
	}); err != nil {
		{
			log.Error("wasm host failed", "err", err)
		}
	}
}
