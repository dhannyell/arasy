//go:build windows && amd64

package main

import (
	"log/slog"
	"os"

	"github.com/dhannyell/frameforge/examples/bench/particlebench"
	"github.com/dhannyell/frameforge/platform/native"
)

func main() {
	game, err := particlebench.NewGame()
	if err != nil {
		panic(err)
	}

	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	if err := native.Run(game, native.Options{
		Title:  "Particle Benchmark",
		Width:  1980,
		Height: 1080,
		Log:    log,
	}); err != nil {
		log.Error("native host error", "err", err)
		os.Exit(1)
	}
}
