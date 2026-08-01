//go:build (windows || linux || darwin) && (amd64 || arm64)

package main

import (
	"os"

	"github.com/dhannyell/arasy/examples/uimanager"
	"github.com/dhannyell/arasy/platform/native"
)

func main() {
	log, closeLog := native.NewLogger("UI Manager Example")
	defer func() { _ = closeLog() }()
	if err := native.Run(&uimanager.Manager{}, native.Options{
		Title:    "UI Manager Example",
		Width:    1920,
		Height:   1080,
		Log:      log,
		DevTools: true,
	}); err != nil {
		log.Error("Error running uimanager.Manager", "error", err)
		os.Exit(1)
	}
}
