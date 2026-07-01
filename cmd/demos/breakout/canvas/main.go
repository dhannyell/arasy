package main

import (
	"log/slog"
	"os"

	"github.com/dhannyell/frameforge/cmd/demos/breakout"
	"github.com/dhannyell/frameforge/platform/web/canvas"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	if err := canvas.Run(&breakout.Breakout{}, canvas.Options{Addr: "127.0.0.1:8080", Log: log}); err != nil {
		log.Error("server failed", "err", err)
		os.Exit(1)
	}
}
