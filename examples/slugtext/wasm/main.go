//go:build js && wasm

package main

import (
	"log/slog"

	"github.com/dhannyell/arasy/examples/slugtext"
	"github.com/dhannyell/arasy/platform/wasm"
	"github.com/dhannyell/arasy/render/slug"
)

func main() {
	log := wasm.NewConsoleLogger(slog.LevelInfo)
	if err := wasm.Run(&slugtext.Game{}, wasm.Options{Log: log, GlyphOutlines: slug.Outlines}); err != nil {
		log.Error("wasm host failed", "err", err)
	}
}
