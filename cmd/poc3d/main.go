package main

import (
	"log/slog"
	"os"

	"github.com/dhannyell/arasy/platform/native"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	if err := native.Run(&poc3d{}, native.Options{
		Title:    "POC3D DEMO",
		Width:    1920,
		Height:   1080,
		Log:      log,
		DevTools: true,
	}); err != nil {
		log.Error("poc3d error", "err", err)
		os.Exit(1)
	}
}
