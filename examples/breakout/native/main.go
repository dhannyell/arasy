//go:build (windows || linux || darwin) && (amd64 || arm64)

package main

import (
	"log/slog"
	"os"

	"github.com/dhannyell/arasy/examples/breakout"
	"github.com/dhannyell/arasy/platform/native"
	"github.com/dhannyell/arasy/runtime"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	if err := native.Run(&breakout.Breakout{}, native.Options{
		Title:        "Breakout",
		Log:          log,
		DevTools:     true,
		RecordReplay: true,
		GameFactory:  func() runtime.Game { return &breakout.Breakout{} },
	}); err != nil {
		log.Error("native host failed", "err", err)
		os.Exit(1)
	}
}
