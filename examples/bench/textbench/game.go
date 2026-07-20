// Package textbench is a browser performance benchmark for the WebGPU text
// render path: per-frame atlas resolution (resolveFrameAtlases grouping text
// by (FontID, pixel size) before tessellation), DrawText tessellation into
// vertex streams, and the textured glyph-atlas draw runs that follow it. It
// keeps its own label state instead of the ECS so results reflect only the
// text path.
package textbench

import (
	"embed"
	"fmt"
	"io/fs"
	"strconv"
	"strings"
	"time"

	"github.com/dhannyell/frameforge/asset"
	"github.com/dhannyell/frameforge/input"
	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
	"github.com/dhannyell/frameforge/runtime"
	"github.com/dhannyell/frameforge/ui"
)

//go:embed assets
var embeddedAssets embed.FS

const (
	worldWidth  = 1920
	worldHeight = 1080

	defaultLabelCount = 500
	minLabelCount     = 1
	maxLabelCount     = 20_000

	gridCols   = 50
	cellWidth  = worldWidth / gridCols
	cellHeight = 22
)

// fontSizes are the four distinct pixel sizes used in multi-size mode. Each
// distinct size requests its own (FontID, PX) atlas, so cycling through them
// forces the per-frame atlas batching to track several atlases at once
// instead of the single shared one used in single-size mode.
var fontSizes = [4]float64{12, 18, 26, 36}

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

// Game lays out N text labels on a deterministic grid and renders one
// render.DrawText per label. Label count and two independent stressors are
// controlled at runtime to explore the text path's scaling behavior:
//
//   - font size mode: a single shared size (one atlas) versus four distinct
//     sizes cycled by label index (four atlases resolved every frame);
//   - text mode: the same label text repeated for every entry versus a
//     unique string per label, which is the worst case for per-frame atlas
//     request batching because no two labels can share a cached glyph run
//     across a naive per-string cache.
type Game struct {
	assets asset.Manifest
	font   render.FontID

	labelCount int
	multiSize  bool
	uniqueText bool

	stats frameStats
}

func NewGame() (*Game, error) {
	sub, err := fs.Sub(embeddedAssets, "assets")
	if err != nil {
		return nil, err
	}
	manifest, err := asset.Load(sub)
	if err != nil {
		return nil, err
	}
	return &Game{
		assets: manifest,
		font:   manifest.Font("default"),
	}, nil
}

func (g *Game) Setup() runtime.Config {
	theme := ui.DefaultTheme()
	theme.Font = g.assets.Font("default")

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
		InputBindings: textbenchBindings(),
	}
}

func (g *Game) Init(*runtime.UpdateContext) {
	g.restart()
}

func (g *Game) restart() {
	g.labelCount = defaultLabelCount
	g.multiSize = false
	g.uniqueText = false
}

func (g *Game) Update(ctx *runtime.UpdateContext, _ float64) {
	if ctx.Input.JustPressed(input.ActionJump) {
		g.labelCount = clampInt(g.labelCount*2, minLabelCount, maxLabelCount)
	}
	if ctx.Input.JustPressed(input.ActionCancel) {
		g.labelCount = clampInt(g.labelCount/2, minLabelCount, maxLabelCount)
	}
	if ctx.Input.JustPressed(input.ActionConfirm) {
		g.multiSize = !g.multiSize
	}
	if ctx.Input.JustPressed(input.ActionFocusNext) {
		g.uniqueText = !g.uniqueText
	}
}

// labelPos places label i on a deterministic row-major grid, wrapping to a
// new row every gridCols entries.
func labelPos(i int) math2d.Vec2 {
	col := i % gridCols
	row := i / gridCols
	return math2d.Vec2{
		X: float64(col)*cellWidth + cellWidth/2,
		Y: float64(row)*cellHeight + cellHeight/2,
	}
}

// labelSize returns the font pixel size for label i: a fixed size in
// single-size mode, or one of four sizes cycled by index in multi-size mode.
func (g *Game) labelSize(i int) float64 {
	if !g.multiSize {
		return fontSizes[0]
	}
	return fontSizes[i%len(fontSizes)]
}

// labelText is a pure function of index so growing/shrinking the label count
// never needs bookkeeping beyond the count itself. Repeated mode returns the
// same HUD-shaped string for every label; unique mode derives a distinct
// player id and score from the index.
func labelText(i int, unique bool) string {
	if !unique {
		return "Player 1234  42 pts"
	}
	playerID := 1000 + i%9000
	score := (i * 37) % 1000
	return fmt.Sprintf("Player %d  %d pts", playerID, score)
}

func (g *Game) Render(ctx *runtime.RenderContext) []render.Command {
	commands := make([]render.Command, 0, g.labelCount)
	for i := 0; i < g.labelCount; i++ {
		commands = append(commands, render.DrawText{
			Position: labelPos(i),
			Text:     labelText(i, g.uniqueText),
			Color:    render.RGBA(0.85, 0.9, 1, 1),
			Size:     g.labelSize(i),
			Font:     g.font,
			Align:    render.AlignCenter,
			Baseline: render.BaselineMiddle,
		})
	}

	hud := textbenchLayout(ctx.Camera().Viewport)
	d := ctx.UI.In(ui.SpaceScreen)
	d.Panel(hud.panel)
	d.Label("TEXT RENDER BENCHMARK", hud.title)
	d.Label(fmt.Sprintf("labels  %s     sizes  %s     text  %s", formatCount(g.labelCount), g.sizeModeLabel(), g.textModeLabel()), hud.load)
	d.Label(fmt.Sprintf("FPS  %5.1f     frame  %6.2f ms     P99  %6.2f ms", g.stats.fps, milliseconds(g.stats.frameTime), milliseconds(g.stats.p99)), hud.frame)
	d.Label(g.gpuLabel(), hud.gpu)
	d.Label(g.cpuLabel(), hud.cpu)
	d.Label(g.cpuBreakdownLabel(), hud.breakdown)
	d.Label("UP doubles, DOWN halves, T toggles 1/4 font sizes, E toggles repeated/unique text", hud.help)
	return commands
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

func (g *Game) sizeModeLabel() string {
	if g.multiSize {
		return "4 (cycled)"
	}
	return "1"
}

func (g *Game) textModeLabel() string {
	if g.uniqueText {
		return "unique"
	}
	return "repeated"
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

func textbenchBindings() []input.Binding {
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

type textbenchHUD struct {
	panel                        math2d.Rect
	title, load, frame, gpu, cpu math2d.Vec2
	breakdown, help              math2d.Vec2
}

func textbenchLayout(viewport math2d.Rect) textbenchHUD {
	w, h := viewport.Width(), viewport.Height()
	panelW := math2d.Clamp(w*0.48, 560, 760)
	panelH := math2d.Clamp(h*0.40, 290, 350)
	x := viewport.Min.X + (w-panelW)/2
	y := viewport.Min.Y + math2d.Clamp(h*0.035, 18, 42)
	line := panelH * 0.09
	return textbenchHUD{
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
