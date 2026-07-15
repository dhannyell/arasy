//go:build js && wasm

package main

import (
	"log/slog"

	"github.com/dhannyell/frameforge/examples/rhythm"
	"github.com/dhannyell/frameforge/platform/wasm"
	"github.com/dhannyell/frameforge/runtime"
)

func main() {
	log := wasm.NewConsoleLogger(slog.LevelInfo)
	game, err := rhythm.NewGame()
	if err != nil {
		log.Error("Failed to create game", "error", err)
	}
	if err := wasm.Run(game, wasm.Options{
		DevTools:      true,
		Log:           log,
		RecordReplay:  true,
		ForceCanvas2D: false,
		GameFactory: func() runtime.Game {
			game, err := rhythm.NewGame()
			if err != nil {
				log.Error("Failed to create game", "error", err)
			}
			return game
		},
	}); err != nil {
		log.Error("wasm host failed", "err", err)
	}
}
