package main

import (
	"github.com/dhannyell/arasy/examples/rhythm"
	"github.com/dhannyell/arasy/platform/raster"
)

func main() {
	game, err := rhythm.NewGame()
	if err != nil {
		panic(err)
	}
	if err := raster.Run(game, raster.Options{
		Width:     1280,
		Height:    960,
		Frames:    1,
		FrameDT:   1.0 / 60.0,
		PNG:       "output/rhythm.png",
		FramesDir: "output/frames",
	}); err != nil {
		panic(err)
	}
	println("wrote output/rhythm.png")
}
