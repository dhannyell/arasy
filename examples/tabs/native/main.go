//go:build (windows || linux || darwin) && (amd64 || arm64)

package main

import (
	"os"

	"github.com/dhannyell/arasy/examples/tabs"
	"github.com/dhannyell/arasy/platform/native"
)

func main() {
	log, closeLog := native.NewLogger("Tabs")
	defer func() { _ = closeLog() }()
	if err := native.Run(&tabs.Game{}, native.Options{
		Title: "Tabs", Width: 960, Height: 540,
		VSync: native.VSyncOn, Log: log,
		DevTools: true,
	}); err != nil {
		log.Error("native host failed", "err", err)
		os.Exit(1)
	}
}
