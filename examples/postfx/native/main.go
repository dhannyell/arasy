//go:build (windows || linux || darwin) && (amd64 || arm64)

package main

import (
	"os"

	"github.com/dhannyell/arasy/examples/postfx"
	"github.com/dhannyell/arasy/internal/brand"
	"github.com/dhannyell/arasy/platform/native"
)

func main() {
	game, err := postfx.NewGame()
	if err != nil {
		panic(err)
	}

	log, closeLog := native.NewLogger("Post FX Demo")
	defer func() { _ = closeLog() }()
	if err := native.Run(game, native.Options{
		Title:    "Post FX Demo",
		Icon:     brand.AppIcon(),
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
