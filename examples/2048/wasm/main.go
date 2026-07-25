//go:build js && wasm

package main

import (
	"log/slog"

	"github.com/dhannyell/arasy/examples/2048"
	"github.com/dhannyell/arasy/platform/wasm"
	"github.com/dhannyell/arasy/runtime"
)

func main() {
	log := wasm.NewConsoleLogger(slog.LevelInfo)
	if err := wasm.Run(&twenty48.Game2048{}, wasm.Options{
		DevTools:     true,
		Log:          log,
		RecordReplay: true,
		GameFactory:  func() runtime.Game { return &twenty48.Game2048{} },
	}); err != nil {
		log.Error("wasm host failed", "err", err)
	}
}
