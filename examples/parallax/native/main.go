package main

import (
	"log/slog"
	"os"

	"github.com/dhannyell/arasy/examples/parallax"
	"github.com/dhannyell/arasy/platform/native"
)

func main() {
	game, err := parallax.NewGame()
	if err != nil {
		panic(err)
	}

	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	if err := native.Run(game, native.Options{
		Title:  "Parallax Layers",
		Width:  1280,
		Height: 720,
		Log:    log,
	}); err != nil {
		log.Error("native host error", "err", err)
	}
}
