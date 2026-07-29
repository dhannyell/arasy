package wasmgolden

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing/fstest"

	"github.com/dhannyell/arasy/asset"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
	"github.com/dhannyell/arasy/runtime"
)

// particleGame is the focused Canvas2D/WebGPU particle regression fixture.
// It stays isolated from the canonical scene so particle backend semantics
// can evolve without rewriting unrelated golden coverage.
type particleGame struct {
	assets      asset.Manifest
	renderCount uint64
}

// NewParticleGolden builds the particle fixture used by the browser golden
// harness.
func NewParticleGolden() (runtime.Game, error) {
	manifest, err := particleGoldenManifest()
	if err != nil {
		return nil, err
	}
	return &particleGame{assets: manifest}, nil
}

func (g *particleGame) Setup() runtime.Config {
	return runtime.Config{
		SimulationHz: 60,
		Camera: render.Camera{
			World:    math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 100, Y: 100}),
			Viewport: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 320, Y: 240}),
			Aspect:   render.AspectStretch,
		},
		Assets:              g.assets,
		DisableDefaultSplash: true,
		Clear:               render.RGBA8(14, 18, 27, 255),
	}
}

func (*particleGame) Init(*runtime.UpdateContext) {}

func (*particleGame) Update(ctx *runtime.UpdateContext, _ float64) {
	ctx.HoldSimulation()
}

func (g *particleGame) Render(ctx *runtime.RenderContext) []render.Command {
	commands := []render.Command{
		panel(1, 3, 48, 43),
		panel(51, 3, 46, 43),
		panel(1, 49, 48, 48),
		panel(51, 49, 46, 48),
	}

	// Frame zero only seeds each emitter's cumulative burst tracker. Frame
	// one spawns and advances exactly one particle per emitter; all later
	// frames use dt zero, making the screenshot independent of when
	// Playwright observes the ready canvas.
	burst := uint32(0)
	if g.renderCount > 0 {
		burst = 1
	}

	gradient := ctx.Assets.Effect("gradient")
	progresses := [...]float64{0.10, 0.30, 0.60, 0.90}
	for i, progress := range progresses {
		dt := 0.0
		if g.renderCount == 1 {
			dt = progress
		}
		commands = append(commands, render.DrawParticles{
			BaseCommand: render.BaseCommand{LayerValue: 1},
			Effect:      gradient,
			Emitter:     render.EmitterID(i + 1),
			Pos:         math2d.Vec2{X: 10 + float64(i)*10, Y: 24},
			Burst:       burst,
			Dt:          dt,
			Seed:        uint64(100 + i),
		})
	}

	duplicateDt := 0.0
	if g.renderCount == 1 {
		duplicateDt = 0.20
	}
	duplicate := render.DrawParticles{
		BaseCommand: render.BaseCommand{LayerValue: 1},
		Effect:      ctx.Assets.Effect("duplicate"),
		Emitter:     10,
		Pos:         math2d.Vec2{X: 25, Y: 72},
		Burst:       burst,
		Dt:          duplicateDt,
		Seed:        200,
	}
	commands = append(commands, duplicate, duplicate)

	rotationDt := 0.0
	if g.renderCount == 1 {
		rotationDt = 0.25
	}
	commands = append(commands, render.DrawParticles{
		BaseCommand: render.BaseCommand{LayerValue: 1},
		Effect:      ctx.Assets.Effect("rotation"),
		Emitter:     20,
		Pos:         math2d.Vec2{X: 74, Y: 24},
		Burst:       burst,
		Dt:          rotationDt,
		Seed:        300,
	})

	missingDt := 0.0
	if g.renderCount == 1 {
		missingDt = 0.25
	}
	commands = append(commands, render.DrawParticles{
		BaseCommand: render.BaseCommand{LayerValue: 1},
		Effect:      ctx.Assets.Effect("missing"),
		Emitter:     30,
		Pos:         math2d.Vec2{X: 74, Y: 72},
		Burst:       burst,
		Dt:          missingDt,
		Seed:        400,
	})

	g.renderCount++
	return commands
}

func panel(x, y, w, h float64) render.DrawRect {
	return render.DrawRect{
		BaseCommand: render.BaseCommand{LayerValue: 0},
		Rect:        math2d.NewRect(math2d.Vec2{X: x, Y: y}, math2d.Vec2{X: w, Y: h}),
		Fill:        render.RGBA8(25, 35, 52, 255),
		Stroke:      render.RGBA8(72, 89, 116, 255),
		StrokeWidth: 0.5,
	}
}

func particleGoldenManifest() (asset.Manifest, error) {
	white := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			white.SetRGBA(x, y, color.RGBA{R: 255, G: 255, B: 255, A: 255})
		}
	}

	var encoded bytes.Buffer
	if err := png.Encode(&encoded, white); err != nil {
		return asset.Manifest{}, err
	}

	const manifest = `{
		"version": 1,
		"textures": [
			{"name":"particle_broken","file":"broken.png"},
			{"name":"particle_white","file":"white.png"}
		],
		"effects": [
			{
				"name":"gradient","texture":"particle_white","capacity":8,
				"emit_rate":0,"burst_size":1,"lifetime":[1,1],"speed":[0,0],
				"size_start":[8,8],"size_end":[8,8],"drag":1,
				"color_start":[1,0.15,0.05,1],"color_end":[0.05,0.35,1,1]
			},
			{
				"name":"duplicate","capacity":8,
				"emit_rate":0,"burst_size":1,"lifetime":[1,1],"speed":[0,0],
				"size_start":[12,12],"size_end":[12,12],"drag":1,
				"color_start":[1,0.1,0.1,1],"color_end":[0.1,1,0.2,1]
			},
			{
				"name":"rotation","capacity":8,
				"emit_rate":0,"burst_size":1,"lifetime":[1,1],"speed":[0,0],
				"size_start":[14,14],"size_end":[14,14],"drag":1,
				"rotation":[45,45],"color_start":[0.95,0.95,0.95,1],
				"color_end":[0.95,0.95,0.95,1]
			},
			{
				"name":"missing","texture":"particle_broken","capacity":8,
				"emit_rate":0,"burst_size":1,"lifetime":[1,1],"speed":[0,0],
				"size_start":[14,14],"size_end":[14,14],"drag":1,
				"color_start":[0,1,1,1],"color_end":[0,1,1,1]
			}
		]
	}`

	return asset.Load(fstest.MapFS{
		"manifest.json": {Data: []byte(manifest), Mode: 0o644},
		"white.png":     {Data: encoded.Bytes(), Mode: 0o644},
		// Deliberately invalid image bytes characterize each backend's missing
		// texture placeholder without using an unknown manifest texture.
		"broken.png": {Data: []byte("not a png"), Mode: 0o644},
	})
}
