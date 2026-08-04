package main

import (
	"github.com/dhannyell/arasy/examples/outpost"
	"github.com/dhannyell/arasy/platform/raster"
)

func main() {
	game, err := outpost.NewGame()
	if err != nil {
		panic(err)
	}
	if err := raster.Run(game, raster.Options{
		Width:   1600,
		Height:  960,
		Frames:  1,
		FrameDT: 1.0 / 60.0,
		PNG:     "output/outpost.png",
	}); err != nil {
		panic(err)
	}
	println("wrote output/outpost.png")
}
