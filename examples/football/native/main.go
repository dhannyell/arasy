//go:build (windows || linux || darwin) && (amd64 || arm64)

package main

import (
	"os"

	"github.com/dhannyell/arasy/examples/football"
	"github.com/dhannyell/arasy/internal/brand"
	"github.com/dhannyell/arasy/platform/native"
	"github.com/dhannyell/arasy/runtime"
)

func main() {
	log, closeLog := native.NewLogger("Football Demo")
	defer func() { _ = closeLog() }()
	if err := native.Run(&football.Football{}, native.Options{
		Title:    "Football Demo",
		Icon:     brand.AppIcon(),
		Width:    1920,
		Height:   1080,
		DevTools: true,
		GameFactory: func() runtime.Game {
			return &football.Football{}
		},
		Log: log,
	}); err != nil {
		os.Exit(1)
	}
}
