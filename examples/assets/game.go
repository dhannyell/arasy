// Package assets demonstrates the data-driven texture pipeline in an offline
package assets

import (
	"embed"
	"image"
	"image/color"
	"io/fs"

	"github.com/dhannyell/frameforge/asset"
	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
	"github.com/dhannyell/frameforge/runtime"
)

//go:embed assets
var assetsFS embed.FS

type Game struct {
	assets asset.Manifest
}

func NewGame() (*Game, error) {
	m, err := loadManifest()
	if err != nil {
		return nil, err
	}
	return &Game{assets: m}, nil
}

func (g *Game) Setup() runtime.Config {
	return runtime.Config{
		Camera: render.Camera{
			World:    math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 100, Y: 75}),
			Viewport: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 1280, Y: 960}),
			Aspect:   render.AspectFit,
		},
		Assets: g.assets,
		Clear:  render.RGBA(0.05, 0.08, 0.10, 1),
	}
}

func (g *Game) Init(*runtime.UpdateContext) {}

func (g *Game) Update(ctx *runtime.UpdateContext, dt float64) {
	ctx.HoldSimulation()
}

var heatmapCount int

func (g *Game) Render(ctx *runtime.RenderContext) []render.Command {
	commands := make([]render.Command, 0, 10)
	heatmapCount++
	heatmapTex := ctx.Assets.Texture("heatmap")
	// Generate a procedural heatmap every frame
	img := image.NewRGBA(image.Rect(0, 0, 16, 16))
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			v := uint8(((x ^ y) * heatmapCount * 8) & 0xff)
			img.SetRGBA(x, y, color.RGBA{v, v ^ 0xff, uint8(x * 16), 255})
		}
	}
	ctx.UpdateTexture(heatmapTex, img)

	ballTexture := ctx.Assets.Texture("ball")
	ballSprite := ctx.Assets.Sprite("ball_left")

	commands = append(commands, render.DrawImage{
		BaseCommand: render.BaseCommand{LayerValue: 1},
		Texture:     ballTexture,
		Dest:        math2d.NewRect(math2d.Vec2{X: 15, Y: 37.5}, math2d.Vec2{X: 10, Y: 10}),
	})

	commands = append(commands, render.DrawImage{
		BaseCommand: render.BaseCommand{LayerValue: 1},
		Texture:     ballSprite.Texture,
		Src:         ballSprite.Src,
		Dest:        math2d.NewRect(math2d.Vec2{X: 32.5, Y: 37.5}, math2d.Vec2{X: 5, Y: 10}),
	})

	commands = append(commands, render.DrawImage{
		BaseCommand: render.BaseCommand{LayerValue: 1},
		Texture:     ballTexture,
		Dest:        math2d.NewRect(math2d.Vec2{X: 45, Y: 37.5}, math2d.Vec2{X: 10, Y: 10}),
		Tint:        render.RGBA(1, 0.4, 0.4, 1),
	})

	commands = append(commands, render.DrawImage{
		BaseCommand: render.BaseCommand{LayerValue: 1},
		Texture:     ballTexture,
		Dest:        math2d.NewRect(math2d.Vec2{X: 60, Y: 37.5}, math2d.Vec2{X: 10, Y: 10}),
		Tint:        render.RGBA(0.5, 0.8, 0.3, 1),
		Rotation:    0.65,
	})

	commands = append(commands, render.DrawImage{
		BaseCommand: render.BaseCommand{LayerValue: 1},
		Texture:     heatmapTex,
		Dest:        math2d.NewRect(math2d.Vec2{X: 0, Y: 0}, math2d.Vec2{X: 16, Y: 16}),
	})

	typoTexture := ctx.Assets.Texture("typo")
	commands = append(commands, render.DrawImage{
		BaseCommand: render.BaseCommand{LayerValue: 1},
		Texture:     typoTexture,
		Dest:        math2d.NewRect(math2d.Vec2{X: 75, Y: 37.5}, math2d.Vec2{X: 10, Y: 10}),
	})

	// A trapezoid quad (narrower top, wider bottom) exercises the projective
	// homography path (DrawImageQuad), which no demo host previously showcased.
	commands = append(commands, render.DrawImageQuad{
		BaseCommand: render.BaseCommand{LayerValue: 1},
		Texture:     ballTexture,
		Corners: [4]math2d.Vec2{
			{X: 42, Y: 52}, {X: 58, Y: 52}, {X: 63, Y: 72}, {X: 37, Y: 72},
		},
	})

	heading := ctx.Assets.Font("heading")
	commands = append(commands,
		render.DrawText{
			BaseCommand: render.BaseCommand{LayerValue: 2},
			Position:    math2d.Vec2{X: 4, Y: 20},
			Text:        "Built-in bitmap font",
			Color:       render.RGBA(0.75, 0.82, 0.9, 1),
			Size:        4,
		},
		render.DrawText{
			BaseCommand: render.BaseCommand{LayerValue: 2},
			Position:    math2d.Vec2{X: 4, Y: 27},
			Text:        "Manifest Go Regular: AV iii MMM",
			Color:       render.RGBA(1, 0.84, 0.35, 1),
			Size:        4,
			Font:        heading,
		},
		render.DrawText{
			BaseCommand: render.BaseCommand{LayerValue: 2},
			Position:    math2d.Vec2{X: 4, Y: 33},
			Text:        "Unknown name falls back to font zero",
			Color:       render.RGBA(0.55, 0.9, 0.65, 1),
			Size:        3,
			Font:        ctx.Assets.Font("missing-font"),
		},
	)

	return commands
}

func loadManifest() (asset.Manifest, error) {
	sub, err := fs.Sub(assetsFS, "assets")
	if err != nil {
		return asset.Manifest{}, err
	}
	return asset.Load(sub)
}
