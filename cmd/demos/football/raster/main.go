package main

import (
	"image"
	"image/color/palette"
	"image/gif"
	"image/png"
	"os"

	"github.com/dhannyell/frameforge/cmd/demos/football"
	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/platform/raster"
	"github.com/dhannyell/frameforge/render"
	"github.com/dhannyell/frameforge/scene"
)

const (
	imageWidth  = 1920
	imageHeight = 1080
	gifWidth    = 1920
	gifHeight   = 1080
	frameCount  = 250
	frameDelay  = 6
	frameDT     = 1.0 / 30.0

	worldWidth  = 115.5
	worldHeight = 74.8
	fieldWidth  = 105.0
	fieldHeight = 68.0
	fieldLeft   = 5.25
	fieldTop    = 3.4
)

var (
	worldOrigin    = math2d.Vec2{X: 0, Y: 0}
	worldSize      = math2d.Vec2{X: worldWidth, Y: worldHeight}
	clearColor     = render.RGBA(0.05, 0.08, 0.10, 1)
	viewportOrigin = math2d.Vec2{X: 0, Y: 0}
	viewportSize   = math2d.Vec2{X: imageWidth, Y: imageHeight}
	fieldOrigin    = math2d.Vec2{X: fieldLeft, Y: fieldTop}
	fieldSize      = math2d.Vec2{X: fieldWidth, Y: fieldHeight}

	colliderColor = render.RGBA(1, 1, 0, 1)
	contactColor  = render.RGBA(1, 0, 1, 1)
)

var debugConfig = football.DebugConfig{
	ShowColliders: true,
	ShowContacts:  true,

	ColliderColor:       colliderColor,
	ColliderStrokeWidth: 0.08,
	ColliderLayer:       100,

	ContactColor:        contactColor,
	ContactPointRadius:  0.18,
	ContactLineWidth:    0.12,
	ContactNormalLength: 1.25,
	ContactLayer:        101,

	ShowGrid:        true,
	GridColor:       render.RGBA(0.5, 0.5, 0.5, 0.4), // cinza semi-transparente
	GridDashLength:  1.5,
	GridStrokeWidth: 0.2,
	GridLayer:       200,

	ShowActiveGrid:        true,
	ActiveGridFill:        render.RGBA(1, 0.55, 0.05, 0.22),
	ActiveGridStroke:      render.RGBA(1, 0.75, 0.05, 0.9),
	ActiveGridStrokeWidth: 0.12,
	ActiveGridLayer:       201,

	ShowCandidateGrid:        true,
	CandidateGridFill:        render.RGBA(1, 0.95, 0.05, 0.35),
	CandidateGridStroke:      render.RGBA(1, 1, 0.15, 1),
	CandidateGridStrokeWidth: 0.18,
	CandidateGridLayer:       202,
}

func main() {
	renderer := raster.Renderer{
		Width:  imageWidth,
		Height: imageHeight,
		Camera: render.Camera{
			World:    math2d.NewRect(worldOrigin, worldSize),
			Viewport: math2d.NewRect(viewportOrigin, viewportSize),
			Aspect:   render.AspectExpand,
		},
		Clear: clearColor,
	}

	world := football.BuildDemoWorld()

	commands := football.BuildFieldCommands()
	commands = append(commands, world.RenderCommands()...)
	football.AppendDebugCommands(&commands, world, debugConfig)

	img := renderer.Render(commands)

	if err := os.MkdirAll("output", 0755); err != nil {
		panic(err)
	}

	if err := savePNG("output/renderdemo.png", img); err != nil {
		panic(err)
	}

	gifRenderer := renderer
	gifRenderer.Width = gifWidth
	gifRenderer.Height = gifHeight
	gifRenderer.Camera.Viewport = math2d.NewRect(
		viewportOrigin,
		math2d.Vec2{X: gifWidth, Y: gifHeight},
	)

	if err := saveGIF("output/renderdemo.gif", gifRenderer, football.BuildDemoWorld()); err != nil {
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

		}
	}(file)

	return png.Encode(file, img)
}

func saveGIF(path string, renderer raster.Renderer, world *scene.World) error {
	animation := &gif.GIF{
		Image: make([]*image.Paletted, 0, frameCount),
		Delay: make([]int, 0, frameCount),
	}

	for range frameCount {
		commands := football.BuildFieldCommands()
		commands = append(commands, world.RenderCommands()...)
		football.AppendDebugCommands(&commands, world, debugConfig)

		img := renderer.Render(commands)
		animation.Image = append(animation.Image, palettedImage(img))
		animation.Delay = append(animation.Delay, frameDelay)

		world.Step(frameDT, scene.StepConfig{
			Bounds: math2d.NewRect(fieldOrigin, fieldSize),
		})
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

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			paletted.Set(x, y, img.At(x, y))
		}
	}

	return paletted
}
