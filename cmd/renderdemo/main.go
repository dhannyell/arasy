package main

import (
	"image/png"
	"os"

	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/platform/raster"
	"github.com/dhannyell/frameforge/render"
)

func main() {
	renderer := raster.Renderer{
		Width:  800,
		Height: 600,
		Camera: render.Camera{
			World: math2d.NewRect(
				math2d.Vec2{X: 0, Y: 0},
				math2d.Vec2{X: 100, Y: 75},
			),
			Viewport: math2d.NewRect(
				math2d.Vec2{X: 0, Y: 0},
				math2d.Vec2{X: 800, Y: 600},
			),
		},
		Clear: render.RGBA(0.05, 0.08, 0.10, 1),
	}

	img := renderer.Render([]render.Command{
		render.DrawRect{
			Rect: math2d.NewRect(
				math2d.Vec2{X: 10, Y: 10},
				math2d.Vec2{X: 80, Y: 55},
			),
			Fill: render.RGBA(0.1, 0.55, 0.2, 1),
		},
		render.DrawRect{
			Rect: math2d.NewRect(
				math2d.Vec2{X: 49, Y: 10},
				math2d.Vec2{X: 2, Y: 55},
			),
			Fill: render.RGBA(0.9, 0.9, 0.9, 1),
		},
		render.DrawLine{
			From:  math2d.Vec2{X: 10, Y: 10},
			To:    math2d.Vec2{X: 90, Y: 10},
			Width: 0.35,
			Color: render.RGBA(1, 1, 1, 1),
		},
		render.DrawLine{
			From:  math2d.Vec2{X: 90, Y: 10},
			To:    math2d.Vec2{X: 90, Y: 65},
			Width: 0.35,
			Color: render.RGBA(1, 1, 1, 1),
		},
		render.DrawLine{
			From:  math2d.Vec2{X: 90, Y: 65},
			To:    math2d.Vec2{X: 10, Y: 65},
			Width: 0.35,
			Color: render.RGBA(1, 1, 1, 1),
		},
		render.DrawLine{
			From:  math2d.Vec2{X: 10, Y: 65},
			To:    math2d.Vec2{X: 10, Y: 10},
			Width: 0.35,
			Color: render.RGBA(1, 1, 1, 1),
		},
		render.DrawLine{
			From:  math2d.Vec2{X: 50, Y: 10},
			To:    math2d.Vec2{X: 50, Y: 65},
			Width: 0.25,
			Color: render.RGBA(1, 1, 1, 1),
		},
		render.DrawRect{
			Rect: math2d.NewRect(
				math2d.Vec2{X: 45, Y: 30},
				math2d.Vec2{X: 10, Y: 10},
			),
			Fill: render.RGBA(1, 1, 1, 1),
		},
		render.DrawCircle{
			Center: math2d.Vec2{X: 30, Y: 35},
			Radius: 2.5,
			Fill:   render.RGBA(0.9, 0.1, 0.1, 1),
		},
		render.DrawCircle{
			Center: math2d.Vec2{X: 35, Y: 28},
			Radius: 2.5,
			Fill:   render.RGBA(0.9, 0.1, 0.1, 1),
		},
		render.DrawCircle{
			Center: math2d.Vec2{X: 70, Y: 35},
			Radius: 2.5,
			Fill:   render.RGBA(0.1, 0.25, 0.95, 1),
		},
		render.DrawCircle{
			Center: math2d.Vec2{X: 65, Y: 42},
			Radius: 2.5,
			Fill:   render.RGBA(0.1, 0.25, 0.95, 1),
		},
		render.DrawCircle{
			Center: math2d.Vec2{X: 52, Y: 36},
			Radius: 1,
			Fill:   render.RGBA(1, 1, 1, 1),
		},
	})

	if err := os.MkdirAll("output", 0755); err != nil {
		panic(err)
	}

	file, err := os.Create("output/renderdemo.png")
	if err != nil {
		panic(err)
	}
	defer file.Close()

	if err := png.Encode(file, img); err != nil {
		panic(err)
	}

	println("wrote output/renderdemo.png")
}
