//go:build js && wasm

package main

import (
	"log/slog"

	"github.com/dhannyell/frameforge/cmd/demos/breakout"
	"github.com/dhannyell/frameforge/platform/wasm"
	"github.com/dhannyell/frameforge/runtime"
)

func main() {
	log := wasm.NewConsoleLogger(slog.LevelInfo)
	if err := wasm.Run(&breakout.Breakout{}, wasm.Options{
		DevTools:      true,
		Log:           log,
		RecordReplay:  true,
		ForceCanvas2D: true,
		GameFactory:   func() runtime.Game { return &breakout.Breakout{} },
	}); err != nil {
		log.Error("wasm host failed", "err", err)
	}
}
