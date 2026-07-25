package main

import (
	"github.com/dhannyell/arasy/examples/2048"
	"github.com/dhannyell/arasy/platform/raster"
)

func main() {
	if err := raster.Run(&twenty48.Game2048{}, raster.Options{
		Width:     720,
		Height:    720,
		Frames:    1,
		FrameDT:   1.0 / 30.0,
		PNG:       "output/twenty48.png",
		FramesDir: "output/frames",
	}); err != nil {
		panic(err)
	}
	println("wrote output/twenty48.png")
}
