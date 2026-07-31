//go:build windows && amd64

package main

import (
	"log/slog"
	"os"

	"github.com/dhannyell/arasy/examples/tabs"
	"github.com/dhannyell/arasy/platform/native"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	if err := native.Run(&tabs.Game{}, native.Options{
		Title: "Tabs", Width: 960, Height: 540,
		VSync: native.VSyncOn, Log: log,
		DevTools: true,
	}); err != nil {
		log.Error("native host failed", "err", err)
		os.Exit(1)
	}
}
