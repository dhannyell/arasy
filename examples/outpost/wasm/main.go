//go:build js && wasm

package main

import (
	"log/slog"

	"github.com/dhannyell/arasy/examples/outpost"
	"github.com/dhannyell/arasy/platform/wasm"
	"github.com/dhannyell/arasy/runtime"
)

func main() {
	game, err := outpost.NewGame()
	if err != nil {
		panic(err)
	}

	log := wasm.NewConsoleLogger(slog.LevelInfo)
	if err := wasm.Run(game, wasm.Options{
		DevTools:     true,
		Log:          log,
		RecordReplay: true,
		GameFactory:  func() runtime.Game { return game },
	}); err != nil {
		log.Error("wasm host failed", "err", err)
	}
}
