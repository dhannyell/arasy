//go:build js && wasm

package main

import (
	"log/slog"

	"github.com/dhannyell/arasy/examples/breakout"
	"github.com/dhannyell/arasy/platform/wasm"
	"github.com/dhannyell/arasy/runtime"
)

func main() {
	log := wasm.NewConsoleLogger(slog.LevelInfo)
	if err := wasm.Run(&breakout.Breakout{}, wasm.Options{
		DevTools:      true,
		Log:           log,
		RecordReplay:  true,
		ForceCanvas2D: false,
		GameFactory:   func() runtime.Game { return &breakout.Breakout{} },
	}); err != nil {
		log.Error("wasm host failed", "err", err)
	}
}
