//go:build (windows || linux || darwin) && (amd64 || arm64)

package main

import (
	"log/slog"
	"os"

	"github.com/dhannyell/arasy/examples/slugtext"
	"github.com/dhannyell/arasy/platform/native"
	"github.com/dhannyell/arasy/render/slug"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	if err := native.Run(&slugtext.Game{}, native.Options{
		Title:  "Slug Text",
		Width:  1280,
		Height: 720,
		Log:    log,

		GlyphOutlines: slug.Outlines,
	}); err != nil {
		log.Error("native host error", "err", err)
	}
}
