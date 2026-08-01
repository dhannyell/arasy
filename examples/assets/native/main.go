//go:build (windows || linux || darwin) && (amd64 || arm64)

package main

import (
	"os"

	"github.com/dhannyell/arasy/examples/assets"
	"github.com/dhannyell/arasy/platform/native"
	"github.com/dhannyell/arasy/runtime"
)

func main() {
	game, err := assets.NewGame()
	if err != nil {
		panic(err)
	}

	log, closeLog := native.NewLogger("Assets Example")
	defer func() { _ = closeLog() }()
	if err := native.Run(game, native.Options{
		Title:        "Assets Example",
		Width:        1920,
		Height:       1080,
		Log:          log,
		DevTools:     true,
		RecordReplay: true,
		// Replay mutates the game it is given, so the factory must build a
		// fresh instance instead of handing back the live one.
		GameFactory: func() runtime.Game {
			replayGame, err := assets.NewGame()
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
