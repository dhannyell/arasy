package main

import (
	"github.com/dhannyell/arasy/examples/groundplane"
	"github.com/dhannyell/arasy/platform/raster"
)

func main() {
	if err := raster.Run(&groundplane.Game{}, raster.Options{
		Width: 960, Height: 540,
		Frames: 180, FrameDT: 1.0 / 60.0,
		PNG: "output/groundplane.png",
		GIF: "output/groundplane.gif",
	}); err != nil {
		panic(err)
	}
}
