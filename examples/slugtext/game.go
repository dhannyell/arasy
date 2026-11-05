// Package slugtext compares the two glyph strategies of DrawText. The left column
// uses the glyph atlas and the right column uses outlines. The large line changes
// size each frame: the atlas rounds its pixel size, and outlines do not.
package slugtext

import (
	"github.com/dhannyell/arasy/dmath"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
	"github.com/dhannyell/arasy/runtime"
)

const (
	worldWidth, worldHeight = 1280.0, 720.0
	atlasColumn             = 40.0
	slugColumn              = 680.0

	retainedLabels render.TextBatchID = 1
)

// Game draws one scene twice, once for each glyph strategy.
type Game struct{}

// Setup returns the camera of the scene.
func (*Game) Setup() runtime.Config {
	screen := math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: worldWidth, Y: worldHeight})
	return runtime.Config{
		Camera:               render.Camera{World: screen, Viewport: screen},
		Clear:                render.RGBA(0.1, 0.16, 0.28, 1),
		SimulationHz:         60,
		DisableDefaultSplash: true,
	}
}

// Init publishes the labels of the retained batch once. The renderer packs them
// again only if the content, the camera, or the glyph storage changes.
func (*Game) Init(ctx *runtime.UpdateContext) {
	ctx.Batches.UpdateText(retainedLabels, []render.TextLabel{{
		Position: math2d.Vec2{X: slugColumn, Y: 520},
		Text:     "A retained batch",
		Size:     36,
		Color:    render.RGBA(0.5, 0.9, 1, 1),
		Strategy: render.GlyphSlug,
	}})
}

// Update has no state to change.
func (*Game) Update(*runtime.UpdateContext, float64) {}

// Render draws the two columns.
func (*Game) Render(ctx *runtime.RenderContext) []render.Command {
	commands := column(atlasColumn, "Atlas", render.GlyphAuto, ctx.Time, nil)
	commands = column(slugColumn, "Slug", render.GlyphSlug, ctx.Time, commands)
	commands = append(commands, render.DrawTextBatch{Batch: retainedLabels})
	return append(commands, render.PostChain{Effects: []render.PostEffect{
		render.Vignette{Color: render.RGBA(0, 0, 0, 1), Strength: 0.35, Softness: 0.7},
	}})
}

// column appends the scene of one strategy at x.
func column(x float64, title string, strategy render.GlyphStrategy, time float64, dst []render.Command) []render.Command {
	draw := func(text string, y, size float64, color render.Color) render.DrawText {
		return render.DrawText{
			Position: math2d.Vec2{X: x, Y: y}, Text: text, Size: size, Color: color, Strategy: strategy,
		}
	}
	white := render.RGBA(1, 1, 1, 1)

	additive := draw("Additive over a rect", 280, 36, render.RGBA(0.9, 0.5, 0.2, 0.9))
	additive.BlendMode = render.BlendAdditive

	clipped := draw("A clip cuts this line", 360, 36, white)
	clipped.Clip = math2d.NewRect(math2d.Vec2{X: x + 60, Y: 335}, math2d.Vec2{X: 300, Y: 18})

	rotated := draw("Rotation", 440, 36, white)
	rotated.Rotation = 0.12

	return append(dst,
		draw(title, 60, 28, render.RGBA(1, 0.8, 0.3, 1)),
		draw("Small text at 14 units: sphinx of black quartz", 100, 14, white),
		draw("Half alpha", 170, 48, render.RGBA(1, 1, 1, 0.5)),
		render.DrawRect{
			Rect: math2d.NewRect(math2d.Vec2{X: x, Y: 245}, math2d.Vec2{X: 360, Y: 50}),
			Fill: render.RGBA(0.25, 0.3, 0.45, 1),
		},
		additive,
		clipped,
		rotated,
		draw("Size changes", 620, 72+16*dmath.Sin(time), white),
	)
}
