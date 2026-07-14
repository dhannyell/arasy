package main

import (
	"github.com/dhannyell/frameforge/examples/football"
	"github.com/dhannyell/frameforge/platform/raster"
)

func main() {
	if err := raster.Run(&football.Football{}, raster.Options{
		Width:     1920,
		Height:    1080,
		Frames:    250,
		FrameDT:   1.0 / 30.0,
		PNG:       "output/renderdemo.png",
		FramesDir: "output/frames",
	}); err != nil {
		panic(err)
	}
	println("wrote output/renderdemo.png")
	println("wrote output/frames/frame-*.png")
}
