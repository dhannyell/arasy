package main

import (
	"log/slog"
	"os"

	"github.com/dhannyell/frameforge/cmd/demos/assets"
	"github.com/dhannyell/frameforge/platform/web/canvas"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	game, err := assets.NewGame()
	if err != nil {
		panic(err)
	}
	err = canvas.Run(game, canvas.Options{Addr: "127.0.0.1:8080", Log: log})
	if err != nil {
		log.Error("server failed", "err", err)
		os.Exit(1)
	}
}
