//go:build js && wasm

package main

import (
	"log/slog"

	"github.com/dhannyell/arasy/examples/splashscreen"
	"github.com/dhannyell/arasy/platform/wasm"
	"github.com/dhannyell/arasy/runtime"
)

func main() {
	game, err := splashscreen.NewGame()
	if err != nil {
		panic(err)
	}

	log := wasm.NewConsoleLogger(slog.LevelInfo)
	if err := wasm.Run(game, wasm.Options{
		DevTools:     true,
		Log:          log,
		RecordReplay: true,
		// Replay mutates the game it is given, so the factory must build a
		// fresh instance instead of handing back the live one.
		GameFactory: func() runtime.Game {
			replayGame, err := splashscreen.NewGame()
			if err != nil {
				panic(err)
			}
			return replayGame
		},
	}); err != nil {
		log.Error("wasm host failed", "err", err)
	}
}
