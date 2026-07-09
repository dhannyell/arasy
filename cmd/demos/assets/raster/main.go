package main

import (
	"github.com/dhannyell/frameforge/cmd/demos/assets"
	"github.com/dhannyell/frameforge/platform/raster"
)

func main() {
	game, err := assets.NewGame()
	if err != nil {
		panic(err)
	}

	if err := raster.Run(game, raster.Options{
		Width:     1920,
		Height:    1080,
		Frames:    5,
		FrameDT:   1.0 / 30.0,
		PNG:       "output/assets-demo.png",
		FramesDir: "output/frames",
	}); err != nil {
		panic(err)
	}
	println("wrote output/assets-demo.png")
	println("wrote output/frames/frame-*.png")
}
