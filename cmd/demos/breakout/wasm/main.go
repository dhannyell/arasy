//go:build js && wasm

package main

import (
	"log/slog"

	"github.com/dhannyell/frameforge/cmd/demos/breakout"
	"github.com/dhannyell/frameforge/platform/wasm"
)

func main() {
	log := wasm.NewConsoleLogger(slog.LevelInfo)
	if err := wasm.Run(&breakout.Breakout{}, wasm.Options{Log: log}); err != nil {
		log.Error("wasm host failed", "err", err)
	}
}
