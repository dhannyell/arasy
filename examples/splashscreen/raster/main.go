package main

import (
	"github.com/dhannyell/arasy/examples/splashscreen"
	"github.com/dhannyell/arasy/platform/raster"
)

func main() {
	game, err := splashscreen.NewGame()
	if err != nil {
		panic(err)
	}

	// The offline raster host never drains the boot splash (it only calls
	// AdvanceTime/RenderFrame): this smoke render always starts at tick 0.
	if err := raster.Run(game, raster.Options{
		Width:     1920,
		Height:    1080,
		Frames:    5,
		FrameDT:   1.0 / 30.0,
		PNG:       "output/splashscreen-demo.png",
		FramesDir: "output/frames",
	}); err != nil {
		panic(err)
	}
	println("wrote output/splashscreen-demo.png")
	println("wrote output/frames/frame-*.png")
}
