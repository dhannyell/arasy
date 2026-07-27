//go:build windows && amd64

package main

import (
	"log/slog"
	"os"

	"github.com/dhannyell/arasy/examples/football"
	"github.com/dhannyell/arasy/platform/native"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	if err := native.Run(&football.Football{}, native.Options{
		Title:  "Football Demo",
		Width:  1920,
		Height: 1080,
		Log:    log,
	}); err != nil {
		os.Exit(1)
	}
}
