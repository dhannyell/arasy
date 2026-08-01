//go:build (windows || linux || darwin) && (amd64 || arm64)

package main

import (
	"os"

	"github.com/dhannyell/arasy/examples/vectoranim"
	"github.com/dhannyell/arasy/internal/brand"
	"github.com/dhannyell/arasy/platform/native"
)

func main() {
	log, closeLog := native.NewLogger("Vector Animation")
	defer func() { _ = closeLog() }()
	if err := native.Run(&vectoranim.Game{}, native.Options{
		Title: "Vector Animation",
		Icon:  brand.AppIcon(),
		Width: 1280, Height: 720,
		VSync: native.VSyncOn, Log: log,
		DevTools: true,
	}); err != nil {
		log.Error("native host failed", "err", err)
		os.Exit(1)
	}
}
