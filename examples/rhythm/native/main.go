//go:build windows && amd64

package main

import (
	"log/slog"
	"os"

	"github.com/dhannyell/arasy/examples/rhythm"
	"github.com/dhannyell/arasy/platform/native"
	"github.com/dhannyell/arasy/runtime"
)

func main() {
	game, err := rhythm.NewGame()
	if err != nil {
		os.Exit(1)
	}

	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	if err := native.Run(game, native.Options{
		Title:        "Rhythm",
		Width:        1920,
		Height:       1080,
		VSync:        native.VSyncOn,
		Log:          log,
		DevTools:     true,
		RecordReplay: true,
		// Replay mutates the game it is given, so the factory must build a
		// fresh instance instead of handing back the live one.
		GameFactory: func() runtime.Game {
			replayGame, err := rhythm.NewGame()
			if err != nil {
				panic(err)
			}
			return replayGame
		},
	}); err != nil {
		log.Error("native host error", "err", err)
		os.Exit(1)
	}
}
