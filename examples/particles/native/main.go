//go:build (windows || linux || darwin) && (amd64 || arm64)

package main

import (
	"os"

	"github.com/dhannyell/arasy/examples/particles"
	"github.com/dhannyell/arasy/platform/native"
)

func main() {
	game, err := particles.NewGame()
	if err != nil {
		panic(err)
	}

	log, closeLog := native.NewLogger("Particles Demo")
	defer func() { _ = closeLog() }()
	if err := native.Run(game, native.Options{
		Title:    "Particles Demo",
		Width:    1920,
		Height:   1080,
		VSync:    native.VSyncOff,
		Log:      log,
		DevTools: true,
	}); err != nil {
		log.Error("native host error", "err", err)
		os.Exit(1)
	}
}
