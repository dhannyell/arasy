//go:build js && wasm

package main

import (
	"log/slog"

	"github.com/dhannyell/frameforge/cmd/demos/assets"
	"github.com/dhannyell/frameforge/platform/wasm"
	"github.com/dhannyell/frameforge/runtime"
)

func main() {
	game, err := assets.NewGame()
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
