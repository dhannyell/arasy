package main

import (
	"log/slog"

	"github.com/dhannyell/arasy/examples/postfx"
	"github.com/dhannyell/arasy/platform/raster"
)

func main() {
	game, err := postfx.NewGame()
	if err != nil {
		panic(err)
	}

	if err := raster.Run(game, raster.Options{
		Width:     1280,
		Height:    720,
		Frames:    120,
		FrameDT:   1.0 / 30.0,
		PNG:       "output/postfx-demo.png",
		FramesDir: "output/postfx-frames",
		Log:       slog.Default(),
	}); err != nil {
		panic(err)
	}
	println("wrote output/postfx-demo.png")
	println("wrote output/postfx-frames/frame-*.png")
}
