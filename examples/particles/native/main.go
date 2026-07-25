//go:build windows && amd64

package main

import (
	"log/slog"
	"os"

	"github.com/dhannyell/arasy/examples/particles"
	"github.com/dhannyell/arasy/platform/native"
)

func main() {
	game, err := particles.NewGame()
	if err != nil {
		panic(err)
	}

	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	if err := native.Run(game, native.Options{
		Title:  "Particles Demo",
		Width:  1920,
		Height: 1080,
		VSync:  native.VSyncOff,
		Log:    log,
	}); err != nil {
		log.Error("native host error", "err", err)
		os.Exit(1)
	}
}
