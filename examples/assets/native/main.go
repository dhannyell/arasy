//go:build windows && amd64

package main

import (
	"log/slog"
	"os"

	"github.com/dhannyell/arasy/examples/assets"
	"github.com/dhannyell/arasy/platform/native"
)

func main() {
	game, err := assets.NewGame()
	if err != nil {
		panic(err)
	}

	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	if err := native.Run(game, native.Options{
		Title:  "Assets Example",
		Width:  1920,
		Height: 1080,
		Log:    log,
	}); err != nil {
		log.Error("native host error", "err", err)
		os.Exit(1)
	}

}
