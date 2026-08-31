package wasmgolden

import (
	"image/color"
	"math"
	"testing/fstest"

	"github.com/dhannyell/arasy/asset"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
	"github.com/dhannyell/arasy/runtime"
)

// flipConformanceGame covers every source-flip state with and without a
// rotation in streaming, static retained and flip-book retained paths.
type flipConformanceGame struct {
	assets asset.Manifest
}

// NewFlipConformanceGolden returns the sprite-flip conformance fixture.
func NewFlipConformanceGolden() (runtime.Game, error) {
	manifest, err := flipConformanceManifest()
	if err != nil {
		return nil, err
	}
	return &flipConformanceGame{assets: manifest}, nil
}

func (g *flipConformanceGame) Setup() runtime.Config {
	return runtime.Config{
		SimulationHz: 60,
		Camera: render.Camera{
			World:    math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 100, Y: 75}),
			Viewport: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 320, Y: 240}),
			Aspect:   render.AspectStretch,
		},
		Assets:               g.assets,
		DisableDefaultSplash: true,
		Clear:                render.RGBA8(12, 16, 24, 255),
	}
}

func (g *flipConformanceGame) Init(ctx *runtime.UpdateContext) {
	texture := g.assets.Texture("quadrants")
	instances := flipInstances(texture, 30, 0, render.SpriteAnimation{})
	instances = append(instances, flipInstances(texture, 40, math.Pi/2, render.SpriteAnimation{})...)
	flipBook := render.SpriteAnimation{Frames: 4, Rate: 1, Phase: 0.5}
	instances = append(instances, flipInstances(texture, 50, 0, flipBook)...)
	instances = append(instances, flipInstances(texture, 60, math.Pi/2, flipBook)...)
	ctx.Batches.UpdateSprites(1, instances)
}

func (*flipConformanceGame) Update(ctx *runtime.UpdateContext, _ float64) {
	ctx.HoldSimulation()
}

func (g *flipConformanceGame) Render(ctx *runtime.RenderContext) []render.Command {
	texture := g.assets.Texture("quadrants")
	commands := flipCommands(texture, 5, 0)
	commands = append(commands, flipCommands(texture, 15, math.Pi/2)...)
	return append(commands, render.DrawSpriteBatch{BaseCommand: render.BaseCommand{LayerValue: 1}, Batch: 1})
}

var flipStates = [...]render.SpriteFlip{render.FlipNone, render.FlipX, render.FlipY, render.FlipX | render.FlipY}

const flipDestSize = 5

func flipCommands(texture render.TextureID, y, rotation float64) []render.Command {
	commands := make([]render.Command, 0, len(flipStates))
	for index, flip := range flipStates {
		commands = append(commands, render.DrawImage{
			Texture:  texture,
			Dest:     math2d.NewRect(math2d.Vec2{X: 10 + float64(index)*10, Y: y}, math2d.Vec2{X: flipDestSize, Y: flipDestSize}),
			Flip:     flip,
			Rotation: rotation,
		})
	}
	return commands
}

func flipInstances(texture render.TextureID, y, rotation float64, animation render.SpriteAnimation) []render.SpriteInstance {
	instances := make([]render.SpriteInstance, 0, len(flipStates))
	for index, flip := range flipStates {
		instance := render.SpriteInstance{
			Texture:   texture,
			Dest:      math2d.NewRect(math2d.Vec2{X: 10 + float64(index)*10, Y: y}, math2d.Vec2{X: flipDestSize, Y: flipDestSize}),
			Flip:      flip,
			Rotation:  rotation,
			Animation: animation,
		}
		if animation.Frames > 1 {
			instance.Src = math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 4, Y: 4})
		}
		instances = append(instances, instance)
	}
	return instances
}

func flipConformanceManifest() (asset.Manifest, error) {
	quadrants, err := encodeGoldenTexture([4]color.RGBA{
		{R: 242, G: 84, B: 105, A: 255},
		{R: 255, G: 213, B: 94, A: 255},
		{R: 105, G: 224, B: 157, A: 255},
		{R: 76, G: 175, B: 255, A: 255},
	})
	if err != nil {
		return asset.Manifest{}, err
	}
	const manifest = `{"version":1,"textures":[{"name":"quadrants","file":"quadrants.png"}]}`
	return asset.Load(fstest.MapFS{
		"manifest.json": {Data: []byte(manifest), Mode: 0o644},
		"quadrants.png": {Data: quadrants, Mode: 0o644},
	})
}
