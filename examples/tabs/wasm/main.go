//go:build js && wasm

package main

import (
	"log/slog"

	"github.com/dhannyell/arasy/examples/tabs"
	"github.com/dhannyell/arasy/platform/wasm"
)

func main() {
	log := wasm.NewConsoleLogger(slog.LevelInfo)
	if err := wasm.Run(&tabs.Game{}, wasm.Options{DevTools: true, Log: log}); err != nil {
		log.Error("wasm host failed", "err", err)
	}
}
