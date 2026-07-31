// Package tabs is the showcase for the ui package's tab widget and titled
// panel: one window-like container whose three tabs each hold a different
// kind of content. It exists to prove the widget end to end — selection in
// Update, drawing in Render, and the caller laying its own content out inside
// the body rect the widget hands back.
package tabs

import (
	"fmt"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
	"github.com/dhannyell/arasy/runtime"
	"github.com/dhannyell/arasy/scene"
	"github.com/dhannyell/arasy/ui"
)

const (
	viewW = 960
	viewH = 540
)

const tabsID = "showcase_tabs"

// State mirrors what the widget reported this tick. The widget owns the
// active tab; the demo only keeps a copy so the golden replay fingerprint has
// something to compare, and so the status line can name the visible tab.
type State struct {
	ActiveTab int
	Switches  int
	Density   float64
}

type Game struct {
	runtime.StateSlot[State]
}

func (g *Game) Setup() runtime.Config {
	world := rect(0, 0, viewW, viewH)
	return runtime.Config{
		Camera:       render.Camera{World: world, Viewport: world, Aspect: render.AspectFit},
		Clear:        render.RGBA8(12, 16, 24, 255),
		Bounds:       world,
		SimulationHz: 60,
		FPS:          60,
		HistoryLen:   240,
	}
}

func (g *Game) Init(*runtime.UpdateContext) {
	g.State = State{Density: 0.35}
}

func (g *Game) Update(ctx *runtime.UpdateContext, _ float64) {
	ctx.HoldSimulation()

	in := ctx.UI.In(ui.SpaceRef)
	active, changed := in.TabsInput(tabsID, tabsRect(), len(tabLabels))
	g.State.ActiveTab = active
	if changed {
		g.State.Switches++
	}

	if active == tabScene {
		g.State.Density, _ = in.SliderInput("density", densityRect(ctx.UI), g.State.Density, ui.SliderOptions{Min: 0, Max: 1})
	}
}

func (g *Game) Render(ctx *runtime.RenderContext) []render.Command {
	draw := ctx.UI.In(ui.SpaceRef)
	draw.TitledPanel(panelRect(), "INSPECTOR")

	active := draw.Tabs(tabsID, tabsRect(), tabLabels)
	body := ctx.UI.TabsBodyRect(tabsRect())
	switch active {
	case tabScene:
		g.drawScene(ctx, draw, body)
	case tabAssets:
		drawAssets(draw, body)
	default:
		drawLog(draw, body)
	}

	draw.Label(fmt.Sprintf("tab %d/%d  ·  %d switches", active+1, len(tabLabels), g.State.Switches),
		math2d.Vec2{X: viewW / 2, Y: viewH - 40})
	return nil
}

func (g *Game) PostStep(*runtime.UpdateContext, scene.StepStats) {}

const (
	tabScene = iota
	tabAssets
	tabLog
)

var tabLabels = []string{"SCENE", "ASSETS", "LOG"}

func (g *Game) drawScene(ctx *runtime.RenderContext, draw ui.DrawScope, body math2d.Rect) {
	draw.Label("Particle density", math2d.Vec2{X: body.Center().X, Y: body.Min.Y + 40})
	draw.Slider("density", densityRect(ctx.UI), g.State.Density, ui.SliderOptions{Min: 0, Max: 1})

	// A bar per tenth of the density value: enough moving pixels for the
	// golden to fail loudly if the body stops tracking the active tab.
	const bars = 10
	gap := 8.0
	width := (body.Width() - 96 - gap*(bars-1)) / bars
	for i := range bars {
		height := 20.0 + float64(i)*6*g.State.Density
		draw.FillRect(
			math2d.NewRect(
				math2d.Vec2{X: body.Min.X + 48 + float64(i)*(width+gap), Y: body.Max.Y - 40 - height},
				math2d.Vec2{X: width, Y: height},
			),
			render.RGBA(0.30, 0.70, 0.95, 0.55),
		)
	}
}

func drawAssets(draw ui.DrawScope, body math2d.Rect) {
	cols := []ui.TableColumn{
		{Title: "NAME", Align: render.AlignLeft},
		{Title: "KIND", Width: 120, Align: render.AlignLeft},
		{Title: "SIZE", Width: 100, Align: render.AlignRight},
	}
	rows := [][3]string{
		{"studio-logo", "texture", "48 KB"},
		{"pixel", "font", "12 KB"},
		{"scene", "vector", "6 KB"},
		{"click", "audio", "3 KB"},
	}
	draw.Table("assets", insetRect(body, 24), cols, len(rows), -1, func(row, col int) string {
		return rows[row][col]
	})
}

func drawLog(draw ui.DrawScope, body math2d.Rect) {
	lines := []string{
		"driver: history 240 ticks",
		"ui: tabs widget ready",
		"render: 0 dropped frames",
	}
	for i, line := range lines {
		draw.DrawText(line,
			math2d.Vec2{X: body.Min.X + 32, Y: body.Min.Y + 44 + float64(i)*32},
			render.RGBA(0.68, 0.72, 0.78, 1), 16,
			render.AlignLeft, render.BaselineMiddle)
	}
}

// tabsGeometry is satisfied by both ui.Interactor and ui.Drawer, so Update and
// Render agree on the body rect without either duplicating the bar height.
type tabsGeometry interface {
	TabsBodyRect(rect math2d.Rect) math2d.Rect
}

func panelRect() math2d.Rect { return rect(120, 60, 720, 400) }

func tabsRect() math2d.Rect {
	panel := panelRect()
	return math2d.NewRect(
		math2d.Vec2{X: panel.Min.X + 24, Y: panel.Min.Y + 56},
		math2d.Vec2{X: panel.Width() - 48, Y: panel.Height() - 80},
	)
}

func densityRect(g tabsGeometry) math2d.Rect {
	body := g.TabsBodyRect(tabsRect())
	return math2d.NewRect(
		math2d.Vec2{X: body.Center().X - 160, Y: body.Min.Y + 70},
		math2d.Vec2{X: 320, Y: 32},
	)
}

func insetRect(r math2d.Rect, inset float64) math2d.Rect {
	return math2d.NewRectFromCorners(
		math2d.Vec2{X: r.Min.X + inset, Y: r.Min.Y + inset},
		math2d.Vec2{X: r.Max.X - inset, Y: r.Max.Y - inset},
	)
}

func rect(x, y, w, h float64) math2d.Rect {
	return math2d.NewRect(math2d.Vec2{X: x, Y: y}, math2d.Vec2{X: w, Y: h})
}
