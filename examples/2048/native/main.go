//go:build windows && amd64

package main

import (
	"log/slog"
	"os"

	twenty48 "github.com/dhannyell/arasy/examples/2048"
	"github.com/dhannyell/arasy/platform/native"
	"github.com/dhannyell/arasy/runtime"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	if err := native.Run(&twenty48.Game2048{}, native.Options{
		Title:        "2048",
		Width:        1920,
		Height:       1080,
		VSync:        native.VSyncOff,
		Log:          log,
		DevTools:     true,
		RecordReplay: true,
		GameFactory:  func() runtime.Game { return &twenty48.Game2048{} },
	}); err != nil {
		log.Error("native host error", "err", err)
		os.Exit(1)
	}
}
