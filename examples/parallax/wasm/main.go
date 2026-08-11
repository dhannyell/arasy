//go:build js && wasm

package main

import (
	"log/slog"

	"github.com/dhannyell/arasy/examples/parallax"
	"github.com/dhannyell/arasy/platform/wasm"
	"github.com/dhannyell/arasy/runtime"
)

func main() {
	game, err := parallax.NewGame()
	if err != nil {
		panic(err)
	}

	log := wasm.NewConsoleLogger(slog.LevelInfo)
	if err := wasm.Run(game, wasm.Options{
		Log:         log,
		GameFactory: func() runtime.Game { return game },
	}); err != nil {
		log.Error("wasm host failed", "err", err)
	}
}
