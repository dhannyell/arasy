package main

import (
	"log/slog"
	"os"

	"github.com/dhannyell/arasy/examples/outpost"
	"github.com/dhannyell/arasy/platform/native"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	if err := native.Run(outpost.NewGame(), native.Options{
		Title:    "Outpost — Isometric Showcase",
		Width:    1600,
		Height:   960,
		Log:      log,
		DevTools: true,
	}); err != nil {
		log.Error("native host error", "err", err)
	}
}
