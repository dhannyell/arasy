package cli

import (
	"fmt"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
	"github.com/dhannyell/arasy/runtime"
)

// stencilGame renders the guide sheet. It is a runtime.Game so the sheet goes
// through the same camera, projection and extrusion a real game's frame does
// -- the guarantee that makes the stencil trustworthy.
type stencilGame struct {
	camera render.Camera
	light  render.ProjectionLight

	projection render.Projection
	upright    math2d.Affine

	// toWorld undoes the view's linear part: it turns "this far on screen"
	// into the world step that lands there, which is how labels and text
	// blocks hold their screen-space layout under any projection.
	toWorld math2d.Affine
}

// The reference scene: a grid of unit tiles with a cube, a lift arrow, a
// floating ball with its projected shadow, and the world axes.
const (
	stencilCols = 6
	stencilRows = 5
)

// stencilContent is the ground rectangle the sheet has to show: the grid plus
// room for the axes beside it and the measurement text below.
func stencilContent() math2d.Rect {
	return math2d.NewRectFromCorners(
		math2d.Vec2{X: -1.6, Y: -1.6},
		math2d.Vec2{X: stencilCols + 1.6, Y: stencilRows + 2.6},
	)
}

// stencilSheetSize is the pixel size that gives the content exactly the asked
// tile scale -- the sheet fits the content instead of the content floating in
// a fixed sheet.
func stencilSheetSize(view math2d.Affine) (int, int) {
	frame := render.FrameGround(view, stencilContent(), 0.04)
	scale := stencilScale(view)
	return int(frame.Width()*scale + 0.5), int(frame.Height()*scale + 0.5)
}

// stencilScale is pixels per view unit: whatever makes one ground tile come
// out --tile pixels wide.
func stencilScale(view math2d.Affine) float64 {
	unit := render.FrameGround(view, math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 1, Y: 1}), 0)
	return stencilTilePx / unit.Width()
}

func (g *stencilGame) Setup() runtime.Config {
	width, height := stencilSheetSize(g.camera.View)
	g.camera.World = render.FrameGround(g.camera.View, stencilContent(), 0.04)
	g.camera.Viewport = math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: float64(width), Y: float64(height)})

	return runtime.Config{
		Camera:               g.camera,
		Clear:                render.RGBA8(24, 26, 40, 255),
		SimulationHz:         60,
		FPS:                  60,
		DisableDefaultSplash: true,
	}
}

func (g *stencilGame) Init(ctx *runtime.UpdateContext) {
	g.projection = render.GroundProjection(ctx.Camera(), g.light)
	if correction, ok := g.projection.UprightCorrection(); ok {
		g.upright = correction
	}

	g.toWorld = math2d.Affine{A: 1, D: 1}
	if view := g.camera.View; !view.IsIdentity() {
		view.E, view.F = 0, 0
		if inverse, ok := view.Inverse(); ok {
			g.toWorld = inverse
		}
	}
}

// onScreen offsets a world point by a view-space vector: "0.5 below on the
// sheet", wherever the projection puts that.
func (g *stencilGame) onScreen(world, offset math2d.Vec2) math2d.Vec2 {
	return world.Add(g.toWorld.Apply(offset))
}

func (g *stencilGame) Update(*runtime.UpdateContext, float64) {}

func (g *stencilGame) Render(*runtime.RenderContext) []render.Command {
	var cmds []render.Command

	cmds = g.appendGrid(cmds)
	cmds = g.appendCube(cmds)
	cmds = g.appendLiftArrow(cmds)
	cmds = g.appendShadowBall(cmds)
	cmds = g.appendAxes(cmds)
	cmds = g.appendMeasurements(cmds)

	return cmds
}

var (
	stencilInk     = render.RGBA8(214, 219, 232, 255)
	stencilFaint   = render.RGBA(1, 1, 1, 0.05)
	stencilLine    = render.RGBA(1, 1, 1, 0.28)
	stencilAccent  = render.RGBA8(255, 213, 122, 255)
	stencilCubeTop = render.RGBA8(170, 176, 192, 255)
	stencilCubeX   = render.RGBA8(96, 102, 120, 255)
	stencilCubeY   = render.RGBA8(128, 134, 152, 255)
)

func (g *stencilGame) appendGrid(cmds []render.Command) []render.Command {
	for j := 0; j < stencilRows; j++ {
		for i := 0; i < stencilCols; i++ {
			fill := render.Color{}
			if (i+j)%2 == 1 {
				fill = stencilFaint
			}
			cmds = append(cmds, render.DrawRect{
				Rect:        math2d.NewRect(math2d.Vec2{X: float64(i), Y: float64(j)}, math2d.Vec2{X: 1, Y: 1}),
				Fill:        fill,
				Stroke:      stencilLine,
				StrokeWidth: 0.02,
			})
		}
	}
	return cmds
}

func (g *stencilGame) appendCube(cmds []render.Command) []render.Command {
	tile := math2d.NewRect(math2d.Vec2{X: 1, Y: 2}, math2d.Vec2{X: 1, Y: 1})
	cmds = g.projection.ExtrudeRect(cmds, tile, 0, 1, render.BlockStyle{
		Top: stencilCubeTop, WallX: stencilCubeX, WallY: stencilCubeY,
	}, 0)
	return g.appendLabel(cmds, g.onScreen(math2d.Vec2{X: 1.5, Y: 3}, math2d.Vec2{Y: 0.55}), "cubo 1 x 1 x 1")
}

func (g *stencilGame) appendLiftArrow(cmds []render.Command) []render.Command {
	foot := math2d.Vec2{X: 4.5, Y: 1.5}
	tip := g.projection.Lift(foot, 1)
	cmds = append(cmds,
		render.DrawLine{From: foot, To: tip, Width: 0.05, Color: stencilAccent},
		render.DrawPoint{Center: tip, Radius: 0.08, Color: stencilAccent},
		render.DrawPoint{Center: foot, Radius: 0.05, Color: stencilAccent},
	)
	return g.appendLabel(cmds, g.onScreen(tip, math2d.Vec2{Y: -0.35}), "1 unidade de altura")
}

func (g *stencilGame) appendShadowBall(cmds []render.Command) []render.Command {
	const radius, height = 0.35, 1.2
	ground := math2d.Vec2{X: 4.5, Y: 3.5}
	projected := g.projection.Project(ground, math2d.Vec2{X: radius, Y: radius}, height)

	if projected.ShadowAlpha > 0 {
		cmds = append(cmds, render.DrawEllipse{
			Center:      ground.Add(projected.ShadowCenter),
			RadiusX:     projected.ShadowRadii.X,
			RadiusY:     projected.ShadowRadii.Y,
			Fill:        render.RGBA(0, 0, 0, projected.ShadowAlpha),
			BaseCommand: render.BaseCommand{Transform: projected.ShadowTransform},
		})
	}
	cmds = append(cmds, render.DrawCircle{
		Center:      projected.BodyPosition,
		Radius:      radius,
		Fill:        stencilAccent,
		BaseCommand: render.BaseCommand{Transform: g.uprightAt(projected.BodyPosition)},
	})
	return g.appendLabel(cmds, g.onScreen(ground, math2d.Vec2{Y: 1.1}), "sombra da engine")
}

func (g *stencilGame) appendAxes(cmds []render.Command) []render.Command {
	origin := math2d.Vec2{X: 0, Y: -0.5}
	for _, axis := range []struct {
		direction math2d.Vec2
		label     string
	}{
		{math2d.Vec2{X: 1}, "+X"},
		{math2d.Vec2{Y: 1}, "+Y"},
	} {
		tip := origin.Add(axis.direction.Mul(1.2))
		cmds = append(cmds,
			render.DrawLine{From: origin, To: tip, Width: 0.05, Color: stencilInk},
			render.DrawPoint{Center: tip, Radius: 0.08, Color: stencilInk},
		)
		cmds = g.appendLabel(cmds, tip.Add(axis.direction.Mul(0.35)), axis.label)
	}
	return cmds
}

func (g *stencilGame) appendMeasurements(cmds []render.Command) []render.Command {
	tile := g.tilePixels()
	lift := g.liftPixels()
	lines := []string{
		fmt.Sprintf("tile de chao: %.0f x %.0f px", tile.X, tile.Y),
		fmt.Sprintf("1 unidade de altura: %.0f px", lift.Len()),
		fmt.Sprintf("luz de (%.1f, %.1f), tilt %.2f", g.light.Direction.X, g.light.Direction.Y, g.light.Tilt),
	}

	// The text block lives in sheet space -- anchored to the frame's bottom
	// left corner, stepping straight down the screen -- because under a
	// skewing view "below the grid" in world terms walks diagonally.
	frame := g.camera.World
	anchor := g.toWorld.Apply(math2d.Vec2{X: frame.Min.X + 0.5, Y: frame.Max.Y - 2.0})
	for n, line := range lines {
		cmds = append(cmds, render.DrawText{
			Position: g.onScreen(anchor, math2d.Vec2{Y: float64(n) * 0.55}),
			Text:     line,
			Size:     0.34,
			Color:    stencilInk,
		})
	}
	return cmds
}

// Text needs no upright transform: the camera projects only its anchor, and
// the glyphs already face the reader.
func (g *stencilGame) appendLabel(cmds []render.Command, at math2d.Vec2, text string) []render.Command {
	return append(cmds, render.DrawText{
		Position: at,
		Text:     text,
		Size:     0.3,
		Color:    stencilInk,
		Align:    render.AlignCenter,
	})
}

// uprightAt is the same correction the engine applies to upright sprites, for
// the shapes -- not the text -- that should face the reader.
func (g *stencilGame) uprightAt(at math2d.Vec2) math2d.Affine {
	if g.upright.IsIdentity() {
		return math2d.Affine{}
	}
	return g.upright.About(at)
}

// tilePixels is the projected size of one ground unit on the finished sheet,
// measured through the camera like everything else: the bounding box of a
// unit square's corners on screen.
func (g *stencilGame) tilePixels() math2d.Vec2 {
	first := g.camera.WorldToScreen(math2d.Vec2{})
	min, max := first, first
	for _, corner := range []math2d.Vec2{{X: 1}, {Y: 1}, {X: 1, Y: 1}} {
		p := g.camera.WorldToScreen(corner)
		min.X, min.Y = minF(min.X, p.X), minF(min.Y, p.Y)
		max.X, max.Y = maxF(max.X, p.X), maxF(max.Y, p.Y)
	}
	return max.Sub(min)
}

func minF(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func maxF(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func (g *stencilGame) liftPixels() math2d.Vec2 {
	ground := math2d.Vec2{X: 1, Y: 1}
	a := g.camera.WorldToScreen(ground)
	b := g.camera.WorldToScreen(g.projection.Lift(ground, 1))
	return b.Sub(a)
}
