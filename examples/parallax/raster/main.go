package main

import (
	"github.com/dhannyell/arasy/examples/parallax"
	"github.com/dhannyell/arasy/platform/raster"
)

func main() {
	game, err := parallax.NewGame()
	if err != nil {
		panic(err)
	}
	if err := raster.Run(game, raster.Options{
		Width:   1280,
		Height:  720,
		Frames:  1,
		FrameDT: 1.0 / 60.0,
		PNG:     "output/parallax.png",
	}); err != nil {
		panic(err)
	}
	println("wrote output/parallax.png")
}
