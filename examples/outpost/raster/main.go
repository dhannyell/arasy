package main

import (
	"github.com/dhannyell/arasy/examples/outpost"
	"github.com/dhannyell/arasy/platform/raster"
)

func main() {
	if err := raster.Run(outpost.NewGame(), raster.Options{
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
