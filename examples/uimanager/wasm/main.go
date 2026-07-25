//go:build js && wasm

package main

import (
	"log/slog"
	"os"

	"github.com/dhannyell/arasy/examples/uimanager"
	"github.com/dhannyell/arasy/platform/wasm"
)

func main() {
	log := wasm.NewConsoleLogger(slog.LevelInfo)
	if err := wasm.Run(&uimanager.Manager{}, wasm.Options{Log: log, DevTools: true}); err != nil {
		log.Error("server failed", "err", err)
		os.Exit(1)
	}
}
