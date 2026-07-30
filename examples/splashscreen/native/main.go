//go:build windows && amd64

package main

import (
	"log/slog"
	"os"

	"github.com/dhannyell/arasy/examples/splashscreen"
	"github.com/dhannyell/arasy/platform/native"
)

func main() {
	game, err := splashscreen.NewGame()
	if err != nil {
		panic(err)
	}

	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	if err := native.Run(game, native.Options{
		Title:  "Splash Screen Example",
		Width:  1280,
		Height: 960,
		Log:    log,
	}); err != nil {
		log.Error("native host error", "err", err)
		os.Exit(1)
	}
}
