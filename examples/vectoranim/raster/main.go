package main

import (
	"github.com/dhannyell/arasy/examples/vectoranim"
	"github.com/dhannyell/arasy/platform/raster"
)

func main() {
	if err := raster.Run(&vectoranim.Game{}, raster.Options{
		Width: 960, Height: 540,
		Frames: 180, FrameDT: 1.0 / 60.0,
		PNG: "output/vectoranim.png",
		GIF: "output/vectoranim.gif",
	}); err != nil {
		panic(err)
	}
}
