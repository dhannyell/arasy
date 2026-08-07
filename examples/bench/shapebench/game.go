// Package shapebench is a browser performance benchmark for the WebGPU
// solid-geometry tessellation path: circles, rectangles, lines and polygons
// tessellated into vertex streams by tess.Build, then the byte conversion and
// upload that follow. Circles are the reference workload for future
// analytic-AA work (signed-distance-field rendering), so they get their own
// dedicated mode alongside the other primitives. It keeps its own shape
// state instead of the ECS so results reflect only the tess/upload path.
package shapebench

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/dhannyell/arasy/asset"
	"github.com/dhannyell/arasy/dmath"
	"github.com/dhannyell/arasy/input"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
	"github.com/dhannyell/arasy/runtime"
	"github.com/dhannyell/arasy/ui"
)

const (
	worldWidth  = 1920
	worldHeight = 1080

	defaultShapeCount = 1000
	minShapeCount     = 1
	maxShapeCount     = 200_000
)

type shapeKind int

const (
	shapeCircle shapeKind = iota
	shapeRect
	shapeLine
	shapePolygon
	shapeMixed
	shapeKindCount
)

func (k shapeKind) label() string {
	switch k {
	case shapeCircle:
		return "circles"
	case shapeRect:
		return "rects"
	case shapeLine:
		return "lines"
	case shapePolygon:
		return "polygons"
	default:
		return "mixed"
	}
}

// resolvedKind returns the concrete shape drawn for index i under mode.
// shapeMixed interleaves the four concrete kinds by index so a mixed frame
// exercises every tessellation path at once.
func resolvedKind(mode shapeKind, i int) shapeKind {
	if mode != shapeMixed {
		return mode
	}
	return shapeKind(i % int(shapeMixed))
}

// shapeState is one bouncing shape: position, velocity and a slow spin.
// Values are a pure function of index so growing/shrinking the active count
// never needs randomness or bookkeeping beyond a slice length.
type shapeState struct {
	Pos      math2d.Vec2
	Vel      math2d.Vec2
	Rotation float64
	Spin     float64
}

func spawnShape(i int) shapeState {
	fi := float64(i)
	x := math.Mod(fi*97.0, worldWidth)
	y := math.Mod(fi*57.0, worldHeight)
	angle := math.Mod(fi*2.399963, 2*math.Pi) // golden-angle spread, deterministic
	speed := 30 + math.Mod(fi*11.3, 100)
	return shapeState{
		Pos:      math2d.Vec2{X: x, Y: y},
		Vel:      math2d.Vec2{X: speed * dmath.Cos(angle), Y: speed * dmath.Sin(angle)},
		Rotation: math.Mod(fi*0.7, 2*math.Pi),
		Spin:     -2 + math.Mod(fi*0.037, 4),
	}
}

// circleRadius cycles 2-50px, deterministic per index.
func circleRadius(i int) float64 {
	return 2 + math.Mod(float64(i)*3.7, 48)
}

// rectHalfExtent cycles a square half-size derived the same way as the
// circle radius, so the two modes cover comparable screen coverage.
func rectHalfExtent(i int) float64 {
	return circleRadius(i)
}

// lineWidth cycles 2-6px, deterministic per index.
func lineWidth(i int) float64 {
	return 2 + math.Mod(float64(i)*0.91, 4)
}

// lineHalfLength is a fixed reach used for both bounce extent and the line
// segment's endpoints.
const lineHalfLength = 20

// polygonSides cycles 5-8, deterministic per index.
func polygonSides(i int) int {
	return 5 + i%4
}

// polygonRadius mirrors circleRadius so polygon and circle modes cover
// comparable screen area.
func polygonRadius(i int) float64 {
	return circleRadius(i)
}

// extentFor returns the half-size used to bounce shape i off the world
// bounds under the given concrete kind.
func extentFor(kind shapeKind, i int) float64 {
	switch kind {
	case shapeCircle:
		return circleRadius(i)
	case shapeRect:
		return rectHalfExtent(i)
	case shapeLine:
		return lineHalfLength
	default: // shapePolygon
		return polygonRadius(i)
	}
}

type frameStats struct {
	fps        float64
	frameTime  time.Duration
	p99        time.Duration
	gpuDraw    time.Duration
	hasGPUTime bool

	cpuTick         time.Duration
	cpuRender       time.Duration
	hasCPUTime      bool
	cpuTess         time.Duration
	cpuUpload       time.Duration
	cpuEncode       time.Duration
	hasCPUBreakdown bool
}

// Game bounces an owned slice of shapes inside the world and renders one
// solid-geometry draw command per shape. Shape count, shape mode and the
// fill/stroke toggle are controlled at runtime to explore the tessellation
// path's scaling behavior across primitive kinds.
type Game struct {
	assets asset.Manifest

	shapes     []shapeState
	shapeCount int
	mode       shapeKind
	withStroke bool

	stats frameStats
}

func (g *Game) Setup() runtime.Config {
	theme := ui.DefaultTheme()

	return runtime.Config{
		Camera: render.Camera{
			World:    math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: worldWidth, Y: worldHeight}),
			Viewport: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: worldWidth, Y: worldHeight}),
			Aspect:   render.AspectExpand,
		},
		SimulationHz:  60,
		FPS:           60,
		Clear:         render.RGBA8(4, 9, 18, 255),
		Assets:        g.assets,
		UITheme:       theme,
		InputBindings: shapebenchBindings(),
	}
}

func (g *Game) Init(*runtime.UpdateContext) {
	g.restart()
}

func (g *Game) restart() {
	g.shapeCount = defaultShapeCount
	g.mode = shapeCircle
	g.withStroke = false
	g.shapes = g.shapes[:0]
	g.growTo(g.shapeCount)
}

func (g *Game) Update(ctx *runtime.UpdateContext, dt float64) {
	if ctx.Input.JustPressed(input.ActionJump) {
		g.setShapeCount(g.shapeCount * 2)
	}
	if ctx.Input.JustPressed(input.ActionCancel) {
		g.setShapeCount(g.shapeCount / 2)
	}
	if ctx.Input.JustPressed(input.ActionConfirm) {
		g.mode = (g.mode + 1) % shapeKindCount
	}
	if ctx.Input.JustPressed(input.ActionFocusNext) {
		g.withStroke = !g.withStroke
	}

	for i := range g.shapes {
		s := &g.shapes[i]
		extent := extentFor(resolvedKind(g.mode, i), i)

		s.Pos.X += s.Vel.X * dt
		s.Pos.Y += s.Vel.Y * dt
		s.Rotation += s.Spin * dt

		if lo := extent; s.Pos.X < lo {
			s.Pos.X = 2*lo - s.Pos.X
			s.Vel.X = -s.Vel.X
		} else if hi := float64(worldWidth) - extent; s.Pos.X > hi {
			s.Pos.X = 2*hi - s.Pos.X
			s.Vel.X = -s.Vel.X
		}
		if lo := extent; s.Pos.Y < lo {
			s.Pos.Y = 2*lo - s.Pos.Y
			s.Vel.Y = -s.Vel.Y
		} else if hi := float64(worldHeight) - extent; s.Pos.Y > hi {
			s.Pos.Y = 2*hi - s.Pos.Y
			s.Vel.Y = -s.Vel.Y
		}
	}
}

func (g *Game) setShapeCount(n int) {
	n = clampInt(n, minShapeCount, maxShapeCount)
	if n == g.shapeCount {
		return
	}
	if n > len(g.shapes) {
		g.growTo(n)
	} else {
		g.shapes = g.shapes[:n]
	}
	g.shapeCount = n
}

func (g *Game) growTo(n int) {
	for i := len(g.shapes); i < n; i++ {
		g.shapes = append(g.shapes, spawnShape(i))
	}
}

var (
	fillColor   = render.RGBA(0.25, 0.75, 0.95, 0.9)
	strokeColor = render.RGBA(0.95, 0.95, 1, 1)
)

func (g *Game) Render(ctx *runtime.RenderContext) []render.Command {
	commands := make([]render.Command, 0, len(g.shapes))
	for i, s := range g.shapes {
		kind := resolvedKind(g.mode, i)
		switch kind {
		case shapeCircle:
			cmd := render.DrawCircle{Center: s.Pos, Radius: circleRadius(i), Fill: fillColor}
			if g.withStroke {
				cmd.Stroke = strokeColor
				cmd.StrokeWidth = 2
			}
			commands = append(commands, cmd)
		case shapeRect:
			half := rectHalfExtent(i)
			rect := math2d.NewRect(
				math2d.Vec2{X: s.Pos.X - half, Y: s.Pos.Y - half},
				math2d.Vec2{X: half * 2, Y: half * 2},
			)
			cmd := render.DrawRect{Rect: rect, Fill: fillColor}
			if g.withStroke {
				cmd.Stroke = strokeColor
				cmd.StrokeWidth = 2
			}
			commands = append(commands, cmd)
		case shapeLine:
			width := lineWidth(i)
			dx := lineHalfLength * math.Cos(s.Rotation)
			dy := lineHalfLength * math.Sin(s.Rotation)
			commands = append(commands, render.DrawLine{
				From:  math2d.Vec2{X: s.Pos.X - dx, Y: s.Pos.Y - dy},
				To:    math2d.Vec2{X: s.Pos.X + dx, Y: s.Pos.Y + dy},
				Width: width,
				Color: fillColor,
			})
		default: // shapePolygon
			cmd := render.DrawPolygon{Points: polygonPoints(s.Pos, polygonRadius(i), s.Rotation, polygonSides(i)), Fill: fillColor}
			if g.withStroke {
				cmd.Stroke = strokeColor
				cmd.StrokeWidth = 2
			}
			commands = append(commands, cmd)
		}
	}

	hud := shapebenchLayout(ctx.Camera().Viewport)
	d := ctx.UI.In(ui.SpaceScreen)
	d.Panel(hud.panel)
	d.Label("SHAPE RENDER BENCHMARK", hud.title)
	d.Label(fmt.Sprintf("shapes  %s     mode  %s     stroke  %s", formatCount(len(g.shapes)), g.mode.label(), g.strokeModeLabel()), hud.load)
	d.Label(fmt.Sprintf("FPS  %5.1f     frame  %6.2f ms     P99  %6.2f ms", g.stats.fps, milliseconds(g.stats.frameTime), milliseconds(g.stats.p99)), hud.frame)
	d.Label(g.gpuLabel(), hud.gpu)
	d.Label(g.cpuLabel(), hud.cpu)
	d.Label(g.cpuBreakdownLabel(), hud.breakdown)
	d.Label("UP doubles, DOWN halves, T cycles circles/rects/lines/polygons/mixed, E toggles fill/stroke", hud.help)
	return commands
}

// polygonPoints places n vertices evenly around center at radius, starting
// at rotation. sides is deterministic per shape (5-8), so the point count
// itself is part of the per-index workload.
func polygonPoints(center math2d.Vec2, radius, rotation float64, sides int) []math2d.Vec2 {
	points := make([]math2d.Vec2, sides)
	for i := 0; i < sides; i++ {
		angle := rotation + 2*math.Pi*float64(i)/float64(sides)
		points[i] = math2d.Vec2{
			X: center.X + radius*math.Cos(angle),
			Y: center.Y + radius*math.Sin(angle),
		}
	}
	return points
}

// ObserveFrame receives host timing after rendering, mirroring the
// spritebench/particlebench pattern.
func (g *Game) ObserveFrame(
	fps float64,
	frameTime, p99, inputLag time.Duration,
	hasInput bool,
	jsHeap, jsHeapMax uint64,
	hasJSHeap bool,
	gpuBuffers uint64,
	hasGPU bool,
	_, _, gpuDraw time.Duration,
	hasGPUTime bool,
	cpuTick, cpuRender time.Duration,
	hasCPUTime bool,
	cpuTess, cpuUpload, cpuEncode time.Duration,
	hasCPUBreakdown bool,
) {
	g.stats = frameStats{
		fps: fps, frameTime: frameTime, p99: p99,
		gpuDraw: gpuDraw, hasGPUTime: hasGPUTime,
		cpuTick: cpuTick, cpuRender: cpuRender, hasCPUTime: hasCPUTime,
		cpuTess: cpuTess, cpuUpload: cpuUpload, cpuEncode: cpuEncode,
		hasCPUBreakdown: hasCPUBreakdown,
	}
}

func (g *Game) strokeModeLabel() string {
	if g.withStroke {
		return "fill+stroke"
	}
	return "fill only"
}

// gpuLabel reports the main render pass's GPU time.
func (g *Game) gpuLabel() string {
	if !g.stats.hasGPUTime {
		return "GPU draw  unavailable (no timestamp-query)"
	}
	return fmt.Sprintf("GPU  draw  %6.2f ms", milliseconds(g.stats.gpuDraw))
}

// cpuLabel reports the measured Go-side wall time of the frame loop, split
// into the simulation tick and the render traversal. Falls back to the
// frame-minus-GPU estimate when the host does not report CPU timings.
func (g *Game) cpuLabel() string {
	if !g.stats.hasCPUTime {
		if !g.stats.hasGPUTime {
			return "cpu timings unavailable"
		}
		cpu := max(g.stats.frameTime-g.stats.gpuDraw, 0)
		return fmt.Sprintf("cpu ~= frame - gpu draw  %6.2f ms", milliseconds(cpu))
	}
	return fmt.Sprintf("CPU  tick  %6.2f ms     render  %6.2f ms", milliseconds(g.stats.cpuTick), milliseconds(g.stats.cpuRender))
}

// cpuBreakdownLabel splits the renderer's share of the render traversal into
// tessellation, vertex upload (JS heap copy + writeBuffer) and command
// encoding. "cmds" is the remainder: the game building its command slice
// plus dispatch/sort.
func (g *Game) cpuBreakdownLabel() string {
	if !g.stats.hasCPUBreakdown {
		return "render breakdown unavailable (canvas2d or no sample yet)"
	}
	cmds := max(g.stats.cpuRender-g.stats.cpuTess-g.stats.cpuUpload-g.stats.cpuEncode, 0)
	return fmt.Sprintf(
		"tess  %6.2f     upload  %6.2f     encode  %6.2f     cmds  %6.2f ms",
		milliseconds(g.stats.cpuTess),
		milliseconds(g.stats.cpuUpload),
		milliseconds(g.stats.cpuEncode),
		milliseconds(cmds),
	)
}

func shapebenchBindings() []input.Binding {
	return []input.Binding{
		{Source: input.Keyboard(input.KeyArrowUp), Action: input.ActionJump, Scale: 1},
		{Source: input.Keyboard(input.KeyArrowDown), Action: input.ActionCancel, Scale: 1},
		{Source: input.Keyboard(input.KeyT), Action: input.ActionConfirm, Scale: 1},
		{Source: input.Keyboard(input.KeyE), Action: input.ActionFocusNext, Scale: 1},
	}
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func milliseconds(d time.Duration) float64 { return float64(d) / float64(time.Millisecond) }

func formatCount(count int) string {
	digits := strconv.Itoa(count)
	if len(digits) <= 3 {
		return digits
	}
	first := len(digits) % 3
	if first == 0 {
		first = 3
	}
	var formatted strings.Builder
	formatted.WriteString(digits[:first])
	for i := first; i < len(digits); i += 3 {
		formatted.WriteByte('.')
		formatted.WriteString(digits[i : i+3])
	}
	return formatted.String()
}

type shapebenchHUD struct {
	panel                        math2d.Rect
	title, load, frame, gpu, cpu math2d.Vec2
	breakdown, help              math2d.Vec2
}

func shapebenchLayout(viewport math2d.Rect) shapebenchHUD {
	w, h := viewport.Width(), viewport.Height()
	panelW := math2d.Clamp(w*0.48, 560, 760)
	panelH := math2d.Clamp(h*0.40, 290, 350)
	x := viewport.Min.X + (w-panelW)/2
	y := viewport.Min.Y + math2d.Clamp(h*0.035, 18, 42)
	line := panelH * 0.09
	return shapebenchHUD{
		panel:     math2d.NewRect(math2d.Vec2{X: x, Y: y}, math2d.Vec2{X: panelW, Y: panelH}),
		title:     math2d.Vec2{X: x + panelW/2, Y: y + line*0.8},
		load:      math2d.Vec2{X: x + panelW/2, Y: y + line*1.9},
		frame:     math2d.Vec2{X: x + panelW/2, Y: y + line*3.0},
		gpu:       math2d.Vec2{X: x + panelW/2, Y: y + line*4.1},
		cpu:       math2d.Vec2{X: x + panelW/2, Y: y + line*5.2},
		breakdown: math2d.Vec2{X: x + panelW/2, Y: y + line*6.3},
		help:      math2d.Vec2{X: x + panelW/2, Y: y + line*7.7},
	}
}
