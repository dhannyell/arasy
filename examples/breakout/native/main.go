//go:build (windows || linux || darwin) && (amd64 || arm64)

package main

import (
	"os"

	"github.com/dhannyell/arasy/examples/breakout"
	"github.com/dhannyell/arasy/internal/brand"
	"github.com/dhannyell/arasy/platform/native"
	"github.com/dhannyell/arasy/runtime"
)

func main() {
	log, closeLog := native.NewLogger("Breakout")
	defer func() { _ = closeLog() }()
	if err := native.Run(&breakout.Breakout{}, native.Options{
		Title:        "Breakout",
		Icon:         brand.AppIcon(),
		Log:          log,
		DevTools:     true,
		RecordReplay: true,
		GameFactory:  func() runtime.Game { return &breakout.Breakout{} },
	}); err != nil {
		log.Error("native host failed", "err", err)
		os.Exit(1)
	}
}
