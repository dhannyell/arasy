// Package scenebench is a browser performance benchmark for the scene.World
// simulation path WITH physics: entities carry Transform, Velocity,
// CircleCollider and CircleSprite components, so every driver tick pays the
// full Step pipeline (acceleration -> integrate -> bounce -> grid broad-phase
// -> contact resolution -> damping), not just render traversal. spritebench
// deliberately excludes colliders to isolate the render path; this benchmark
// deliberately includes them to isolate the broad-phase/contact cost.
package scenebench

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
	"github.com/dhannyell/arasy/scene"
	"github.com/dhannyell/arasy/ui"
)

const (
	worldWidth  = 1920
	worldHeight = 1080

	defaultEntityCount = 1000
	minEntityCount     = 1
	maxEntityCount     = 100_000

	entityRadius    = 5
	physicsCellSize = 64
)

// initialState is one entity's starting transform and velocity. Values are a
// pure function of index so growing/shrinking the active count never needs
// randomness or bookkeeping beyond the entity slice.
type initialState struct {
	Pos math2d.Vec2
	Vel math2d.Vec2
}

func spawnState(i int) initialState {
	fi := float64(i)
	x := math.Mod(fi*97.0, worldWidth-2*entityRadius) + entityRadius
	y := math.Mod(fi*57.0, worldHeight-2*entityRadius) + entityRadius
	angle := math.Mod(fi*2.399963, 2*math.Pi) // golden-angle spread, deterministic
	speed := 40 + math.Mod(fi*13.7, 120)
	return initialState{
		Pos: math2d.Vec2{X: x, Y: y},
		Vel: math2d.Vec2{X: speed * dmath.Cos(angle), Y: speed * dmath.Sin(angle)},
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

	broadPhasePairs int
	contacts        int
}

// Game spawns N entities into scene.World with Transform + Velocity +
// CircleSprite, moved and bounced by the engine's own Step pipeline (not
// mirrored from a slice), so the tick cost measured is the real scene.Step
// cost. Entity count and whether entities carry a CircleCollider are
// controlled at runtime: toggling colliders at the same count isolates the
// grid broad-phase cost from the rest of the tick.
type Game struct {
	assets asset.Manifest

	entities    []scene.EntityID
	entityCount int
	collidersOn bool

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
		Bounds:        math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: worldWidth, Y: worldHeight}),
		Physics:       scene.PhysicsConfig{CellSize: physicsCellSize},
		Assets:        g.assets,
		UITheme:       theme,
		InputBindings: scenebenchBindings(),
	}
}

func (g *Game) Init(ctx *runtime.UpdateContext) {
	g.entityCount = defaultEntityCount
	g.collidersOn = true
	g.spawnEntities(ctx.World)
}

func (g *Game) Update(ctx *runtime.UpdateContext, _ float64) {
	if ctx.Input.JustPressed(input.ActionJump) {
		g.setEntityCount(ctx.World, g.entityCount*2)
	}
	if ctx.Input.JustPressed(input.ActionCancel) {
		g.setEntityCount(ctx.World, g.entityCount/2)
	}
	if ctx.Input.JustPressed(input.ActionFocusNext) {
		g.collidersOn = !g.collidersOn
		g.setColliders(ctx.World, g.collidersOn)
	}
	// Movement, bounce and broad-phase all run through the driver's automatic
	// World.Step after Update returns -- this game does not mirror positions
	// from a slice, unlike spritebench's direct render path.
}

// PostStep captures the tick's broad-phase and contact counts so the HUD can
// show the collision cost the same tick paid, mirroring how PostStepper
// consumers react to the driver's automatic World.Step.
func (g *Game) PostStep(_ *runtime.UpdateContext, stats scene.StepStats) {
	g.stats.broadPhasePairs = stats.BroadPhasePairs
	g.stats.contacts = stats.Contacts
}

func (g *Game) setEntityCount(w *scene.World, n int) {
	n = clampInt(n, minEntityCount, maxEntityCount)
	if n == g.entityCount {
		return
	}
	g.entityCount = n
	// Count changes are rare user events; a full despawn+respawn keeps the
	// entity list trivially in sync. DespawnBatch removes the whole group in
	// one O(n) compaction pass per component store instead of the O(k*n) cost
	// of despawning entities one at a time.
	g.despawnEntities(w)
	g.spawnEntities(w)
}

func (g *Game) spawnEntities(w *scene.World) {
	g.entities = make([]scene.EntityID, 0, g.entityCount)
	for i := 0; i < g.entityCount; i++ {
		state := spawnState(i)
		bundle := scene.Bundle{
			Transform: &scene.Transform{Position: state.Pos, Scale: math2d.Vec2{X: 1, Y: 1}},
			Velocity:  &scene.Velocity{Linear: state.Vel},
			CircleSprite: &scene.CircleSprite{
				Radius: entityRadius,
				BaseSprite: scene.BaseSprite{
					Color: render.RGBA(0.4, 0.85, 0.9, 1),
					Layer: 1,
				},
			},
			Body: &scene.Body{InvMass: 1, Restitution: 1},
		}
		if g.collidersOn {
			bundle.CircleCollider = &scene.CircleCollider{Radius: entityRadius}
		}
		g.entities = append(g.entities, w.SpawnBundle(bundle))
	}
}

// despawnEntities removes the whole entity group in one DespawnBatch call.
// DespawnBatch compacts each component store once (O(n) per store) instead
// of repeatedly shifting dense data with per-entity Despawn calls, which
// would cost O(k*n) for k removed entities out of n live ones.
func (g *Game) despawnEntities(w *scene.World) {
	w.DespawnBatch(g.entities)
	g.entities = g.entities[:0]
}

func (g *Game) setColliders(w *scene.World, on bool) {
	for _, id := range g.entities {
		if on {
			w.SetCircleCollider(id, scene.CircleCollider{Radius: entityRadius})
		} else {
			w.RemoveCircleCollider(id)
		}
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

func (g *Game) Render(ctx *runtime.RenderContext) []render.Command {
	commands := ctx.World.RenderCommands()

	hud := scenebenchLayout(ctx.Camera().Viewport)
	d := ctx.UI.In(ui.SpaceScreen)
	d.Panel(hud.panel)
	d.Label("SCENE SIMULATION BENCHMARK", hud.title)
	d.Label(fmt.Sprintf("entities  %s     colliders  %s", formatCount(g.entityCount), g.colliderModeLabel()), hud.load)
	d.Label(fmt.Sprintf("FPS  %5.1f     frame  %6.2f ms     P99  %6.2f ms", g.stats.fps, milliseconds(g.stats.frameTime), milliseconds(g.stats.p99)), hud.frame)
	d.Label(fmt.Sprintf("broad-phase pairs  %s     contacts  %s", formatCount(g.stats.broadPhasePairs), formatCount(g.stats.contacts)), hud.broadPhase)
	d.Label(g.gpuLabel(), hud.gpu)
	d.Label(g.cpuLabel(), hud.cpu)
	d.Label(g.cpuBreakdownLabel(), hud.breakdown)
	d.Label("UP doubles, DOWN halves, E toggles colliders on/off at the same count", hud.help)
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
	g.stats.fps = fps
	g.stats.frameTime = frameTime
	g.stats.p99 = p99
	g.stats.gpuDraw = gpuDraw
	g.stats.hasGPUTime = hasGPUTime
	g.stats.cpuTick = cpuTick
	g.stats.cpuRender = cpuRender
	g.stats.hasCPUTime = hasCPUTime
	g.stats.cpuTess = cpuTess
	g.stats.cpuUpload = cpuUpload
	g.stats.cpuEncode = cpuEncode
	g.stats.hasCPUBreakdown = hasCPUBreakdown
}

func (g *Game) colliderModeLabel() string {
	if g.collidersOn {
		return "on"
	}
	return "off"
}

// gpuLabel reports the main render pass's GPU time.
func (g *Game) gpuLabel() string {
	if !g.stats.hasGPUTime {
		return "GPU draw  unavailable (no timestamp-query)"
	}
	return fmt.Sprintf("GPU  draw  %6.2f ms", milliseconds(g.stats.gpuDraw))
}

// cpuLabel reports the measured Go-side wall time of the frame loop, split
// into the simulation tick (which pays the full scene.Step cost, including
// broad-phase, when colliders are on) and the render traversal. Falls back
// to the frame-minus-GPU estimate when the host does not report CPU timings.
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
// encoding. "cmds" is the remainder: scene.World extracting render poses
// (RenderCommands id-sort/command building) plus dispatch.
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

func scenebenchBindings() []input.Binding {
	return []input.Binding{
		{Source: input.Keyboard(input.KeyArrowUp), Action: input.ActionJump, Scale: 1},
		{Source: input.Keyboard(input.KeyArrowDown), Action: input.ActionCancel, Scale: 1},
		{Source: input.Keyboard(input.KeyE), Action: input.ActionFocusNext, Scale: 1},
	}
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

type scenebenchHUD struct {
	panel                        math2d.Rect
	title, load, frame, gpu, cpu math2d.Vec2
	broadPhase, breakdown, help  math2d.Vec2
}

func scenebenchLayout(viewport math2d.Rect) scenebenchHUD {
	w, h := viewport.Width(), viewport.Height()
	panelW := math2d.Clamp(w*0.48, 560, 760)
	panelH := math2d.Clamp(h*0.44, 310, 380)
	x := viewport.Min.X + (w-panelW)/2
	y := viewport.Min.Y + math2d.Clamp(h*0.035, 18, 42)
	line := panelH * 0.08
	return scenebenchHUD{
		panel:      math2d.NewRect(math2d.Vec2{X: x, Y: y}, math2d.Vec2{X: panelW, Y: panelH}),
		title:      math2d.Vec2{X: x + panelW/2, Y: y + line*0.7},
		load:       math2d.Vec2{X: x + panelW/2, Y: y + line*1.7},
		frame:      math2d.Vec2{X: x + panelW/2, Y: y + line*2.7},
		broadPhase: math2d.Vec2{X: x + panelW/2, Y: y + line*3.7},
		gpu:        math2d.Vec2{X: x + panelW/2, Y: y + line*4.7},
		cpu:        math2d.Vec2{X: x + panelW/2, Y: y + line*5.7},
		breakdown:  math2d.Vec2{X: x + panelW/2, Y: y + line*6.7},
		help:       math2d.Vec2{X: x + panelW/2, Y: y + line*8.0},
	}
}
