//go:build js && wasm

package main

import (
	"log/slog"

	"github.com/dhannyell/frameforge/examples/rhythm"
	"github.com/dhannyell/frameforge/platform/wasm"
	"github.com/dhannyell/frameforge/runtime"
)

func main() {
	game, err := rhythm.NewGame()
	if err != nil {
		panic(err)
	}

	log := wasm.NewConsoleLogger(slog.LevelInfo)
	if err := wasm.Run(game, wasm.Options{
		DevTools:      true,
		Log:           log,
		RecordReplay:  true,
		ForceCanvas2D: false,
		GameFactory: func() runtime.Game {
			replayGame, err := rhythm.NewGame()
			if err != nil {
				panic(err)
			}
			return replayGame
		},
	}); err != nil {
		log.Error("wasm host failed", "err", err)
	}
}
