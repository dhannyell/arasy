//go:build windows && amd64

package main

import (
	"log/slog"
	"os"

	"github.com/dhannyell/arasy/examples/breakout"
	"github.com/dhannyell/arasy/platform/native"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	if err := native.Run(&breakout.Breakout{}, native.Options{
		Title: "Breakout",
		Log:   log,
	}); err != nil {
		log.Error("native host failed", "err", err)
		os.Exit(1)
	}
}
