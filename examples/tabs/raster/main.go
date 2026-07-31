package main

import (
	"github.com/dhannyell/arasy/examples/tabs"
	"github.com/dhannyell/arasy/platform/raster"
)

func main() {
	if err := raster.Run(&tabs.Game{}, raster.Options{
		Width: 960, Height: 540,
		Frames: 1, FrameDT: 1.0 / 60.0,
		PNG: "output/tabs.png",
	}); err != nil {
		panic(err)
	}
	println("wrote output/tabs.png")
}
