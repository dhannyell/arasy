//go:build windows && amd64

package main

import (
	"log/slog"
	"os"

	"github.com/dhannyell/arasy/examples/rhythm"
	"github.com/dhannyell/arasy/platform/native"
)

func main() {
	game, err := rhythm.NewGame()
	if err != nil {
		os.Exit(1)
	}

	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	if err := native.Run(game, native.Options{
		Title:  "Rhythm",
		Width:  1920,
		Height: 1080,
		VSync:  native.VSyncOn,
		Log:    log,
	}); err != nil {
		log.Error("native host error", "err", err)
		os.Exit(1)
	}
}
