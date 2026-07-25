package main

import (
	"log/slog"

	"github.com/dhannyell/arasy/examples/particles"
	"github.com/dhannyell/arasy/platform/raster"
)

func main() {
	game, err := particles.NewGame()
	if err != nil {
		panic(err)
	}

	log := slog.Default()
	if err := raster.Run(game, raster.Options{
		Width:     3840,
		Height:    2160,
		Frames:    200,
		FrameDT:   1.0 / 30.0,
		PNG:       "output/assets-demo.png",
		FramesDir: "output/frames",
		Log:       log,
	}); err != nil {
		panic(err)
	}
	println("wrote output/particles-demo.png")
	println("wrote output/frames/frame-*.png")
}
