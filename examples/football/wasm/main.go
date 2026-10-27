//go:build js && wasm

package main

import (
	"log/slog"

	"github.com/dhannyell/arasy/examples/football"
	"github.com/dhannyell/arasy/platform/wasm"
	"github.com/dhannyell/arasy/runtime"
)

func main() {
	log := wasm.NewConsoleLogger(slog.LevelInfo)
	if err := wasm.Run(&football.Football{}, wasm.Options{
		DevTools:    true,
		Log:         log,
		GameFactory: func() runtime.Game { return &football.Football{} },
	}); err != nil {
		log.Error("wasm host failed", "err", err)
	}
}
