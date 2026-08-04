package main

import (
	"log/slog"
	"os"

	"github.com/dhannyell/arasy/examples/outpost"
	"github.com/dhannyell/arasy/platform/native"
)

func main() {
	game, err := outpost.NewGame()
	if err != nil {
		panic(err)
	}

	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	if err := native.Run(game, native.Options{
		Title:    "Outpost — Isometric Showcase",
		Width:    1600,
		Height:   960,
		Log:      log,
		DevTools: true,
	}); err != nil {
		log.Error("native host error", "err", err)
	}
}
