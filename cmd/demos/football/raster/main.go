package main

import (
	"image"
	"image/color/palette"
	"image/draw"
	"image/gif"
	"image/png"
	"log/slog"
	"os"

	"github.com/dhannyell/frameforge/cmd/demos/football"
	"github.com/dhannyell/frameforge/platform/raster"
	"github.com/dhannyell/frameforge/runtime"
)

const (
	imageWidth  = 1920
	imageHeight = 1080
	gifWidth    = 1920
	gifHeight   = 1080
	frameCount  = 250
	frameDelay  = 6
	frameDT     = 1.0 / 30.0
)

func main() {
	game := &football.Football{}
	driver := runtime.NewDriver(game, slog.Default())
	cfg := driver.Config()

	renderer := raster.Renderer{
		Width:  imageWidth,
		Height: imageHeight,
		Camera: cfg.Camera,
		Clear:  cfg.Clear,
	}

	img := renderer.Render(driver.Frame().Commands)

	if err := os.MkdirAll("output", 0755); err != nil {
		panic(err)
	}

	if err := savePNG("output/renderdemo.png", img); err != nil {
		panic(err)
	}

	gifRenderer := renderer
	gifRenderer.Width = gifWidth
	gifRenderer.Height = gifHeight

	if err := saveGIF("output/renderdemo.gif", gifRenderer, driver); err != nil {
		panic(err)
	}

	println("wrote output/renderdemo.png")
	println("wrote output/renderdemo.gif")
}

func savePNG(path string, img image.Image) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			println("warning: failed to close file:", err.Error())
		}
	}(file)

	return png.Encode(file, img)
}

func saveGIF(path string, renderer raster.Renderer, driver *runtime.Driver) error {
	animation := &gif.GIF{
		Image: make([]*image.Paletted, 0, frameCount),
		Delay: make([]int, 0, frameCount),
	}

	for range frameCount {
		frame := driver.Frame()
		commands := frame.Commands
		img := renderer.Render(commands)
		animation.Image = append(animation.Image, palettedImage(img))
		animation.Delay = append(animation.Delay, frameDelay)

		driver.Advance(frameDT)
	}

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
		}
	}(file)

	return gif.EncodeAll(file, animation)
}

func palettedImage(img image.Image) *image.Paletted {
	bounds := img.Bounds()
	paletted := image.NewPaletted(bounds, palette.Plan9)

	draw.FloydSteinberg.Draw(paletted, bounds, img, image.Point{})

	return paletted
}
