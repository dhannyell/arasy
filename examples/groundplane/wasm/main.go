//go:build js && wasm

package main

import (
	"log/slog"

	"github.com/dhannyell/arasy/examples/groundplane"
	"github.com/dhannyell/arasy/platform/wasm"
)

func main() {
	log := wasm.NewConsoleLogger(slog.LevelInfo)
	if err := wasm.Run(&groundplane.Game{}, wasm.Options{DevTools: true, Log: log}); err != nil {
		log.Error("wasm host failed", "err", err)
	}
}
