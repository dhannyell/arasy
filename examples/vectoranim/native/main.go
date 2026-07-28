//go:build windows && amd64

package main

import (
	"log/slog"
	"os"

	"github.com/dhannyell/arasy/examples/vectoranim"
	"github.com/dhannyell/arasy/platform/native"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	if err := native.Run(&vectoranim.Game{}, native.Options{
		Title: "Vector Animation", Width: 1280, Height: 720,
		VSync: native.VSyncOn, Log: log,
	}); err != nil {
		log.Error("native host failed", "err", err)
		os.Exit(1)
	}
}
