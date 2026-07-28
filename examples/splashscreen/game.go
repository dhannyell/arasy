// Package splashscreen demonstrates the boot splash playlist: the engine's
// own default segment (a compiled vector brand animation, internal/
// enginesplash) plays first, followed by a developer-declared segment from
// the manifest's "splash" section — a texture flip-book packed with
// `arasy atlas` (see assets/textures/studio-logo.png). Both play before tick
// 0; neither touches the game's own Update/Render.
package splashscreen

import (
	"embed"
	"io/fs"

	"github.com/dhannyell/arasy/asset"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
	"github.com/dhannyell/arasy/runtime"
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
		Clear:  render.RGBA(0.02, 0.04, 0.08, 1),
		// DisableEngineSplash is left at its zero value (false): the engine
		// segment plays, then the manifest's "studio-logo" segment, then
		// tick 0. Set it to true to skip straight to the developer segment.
	}
}

func (g *Game) Init(*runtime.UpdateContext) {}

func (g *Game) Update(ctx *runtime.UpdateContext, dt float64) {
	ctx.HoldSimulation()
}

func (g *Game) Render(ctx *runtime.RenderContext) []render.Command {
	return []render.Command{
		render.DrawText{
			BaseCommand: render.BaseCommand{LayerValue: 1},
			Position:    math2d.Vec2{X: 10, Y: 35},
			Text:        "Boot splash finished -- this is tick 0",
			Color:       render.RGBA(0.85, 0.9, 0.95, 1),
			Size:        4,
		},
		render.DrawText{
			BaseCommand: render.BaseCommand{LayerValue: 1},
			Position:    math2d.Vec2{X: 10, Y: 42},
			Text:        "Playlist: engine default + manifest \"studio-logo\"",
			Color:       render.RGBA(0.6, 0.68, 0.78, 1),
			Size:        3,
		},
	}
}

func loadManifest() (asset.Manifest, error) {
	sub, err := fs.Sub(assetsFS, "assets")
	if err != nil {
		return asset.Manifest{}, err
	}
	return asset.Load(sub)
}
