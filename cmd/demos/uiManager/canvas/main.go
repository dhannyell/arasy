package main

import (
	"log/slog"
	"os"

	uiManager "github.com/dhannyell/frameforge/cmd/demos/uiManager"
	"github.com/dhannyell/frameforge/platform/web/canvas"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	if err := canvas.Run(&uiManager.Manager{}, canvas.Options{Addr: "127.0.0.1:8080", Log: log}); err != nil {
		log.Error("server failed", "err", err)
		os.Exit(1)
	}
}
