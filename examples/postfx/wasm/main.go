//go:build js && wasm

package main

import (
	"log/slog"

	"github.com/dhannyell/arasy/examples/postfx"
	"github.com/dhannyell/arasy/platform/wasm"
)

func main() {
	game, err := postfx.NewGame()
	if err != nil {
		panic(err)
	}

	log := wasm.NewConsoleLogger(slog.LevelInfo)
	if err := wasm.Run(game, wasm.Options{
		ForceCanvas2D: false,
		DevTools:      true,
		Log:           log,
	}); err != nil {
		log.Error("wasm host failed", "err", err)
	}
}
