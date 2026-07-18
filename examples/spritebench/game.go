// Package spritebench is a browser performance benchmark for the WebGPU
// sprite render path: tess.Build tessellating render.DrawImage commands into
// vertex streams, and the per-frame byte conversion and JS-heap upload that
// follow it. It keeps its own sprite state instead of the ECS so results
// reflect only the tess/upload path, not scene.World bookkeeping.
package spritebench

import (
	"embed"
	"fmt"
	"io/fs"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/dhannyell/frameforge/asset"
	"github.com/dhannyell/frameforge/input"
	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
	"github.com/dhannyell/frameforge/runtime"
	"github.com/dhannyell/frameforge/scene"
	"github.com/dhannyell/frameforge/ui"
)

//go:embed assets
var embeddedAssets embed.FS

const (
	worldWidth  = 1920
	worldHeight = 1080
	spriteSize  = 12

	defaultSpriteCount = 1000
	minSpriteCount     = 1
	maxSpriteCount     = 500_000
	textureCount       = 4
)

// spriteState is one bouncing sprite: position, velocity and a slow spin.
// Values are a pure function of index so growing/shrinking the active count
// never needs randomness or bookkeeping beyond a slice length.
type spriteState struct {
	Pos      math2d.Vec2
	Vel      math2d.Vec2
	Rotation float64
	Spin     float64
}

func spawnSprite(i int) spriteState {
	fi := float64(i)
	x := math.Mod(fi*97.0, worldWidth-spriteSize)
	y := math.Mod(fi*57.0, worldHeight-spriteSize)
	angle := math.Mod(fi*2.399963, 2*math.Pi) // golden-angle spread, deterministic
	speed := 40 + math.Mod(fi*13.7, 120)
	return spriteState{
		Pos:      math2d.Vec2{X: x, Y: y},
		Vel:      math2d.Vec2{X: speed * math.Cos(angle), Y: speed * math.Sin(angle)},
		Rotation: math.Mod(fi*0.7, 2*math.Pi),
		Spin:     -2 + math.Mod(fi*0.037, 4),
	}
}

type frameStats struct {
	fps        float64
	frameTime  time.Duration
	p99        time.Duration
	gpuDraw    time.Duration
	hasGPUTime bool
}

// Game bounces an owned slice of sprites inside the world and renders one
// render.DrawImage per sprite. Sprite count and texture-variety mode are
// controlled at runtime to explore the tess/upload path's scaling behavior.
//
// Two render paths are toggleable at the same sprite count:
//   - direct (default): commands are built straight from the sprite slice,
//     isolating the tess/upload path from any engine bookkeeping;
//   - ECS: each sprite is mirrored into a scene.World entity
//     (Transform + ImageSprite, no colliders) and commands come from
//     World.RenderCommands, so the frame additionally pays the per-tick
//     SetTransform store writes, the SyncRenderState transform clone and the
//     RenderCommands id-sort/command building. The delta between the two
//     modes at the same count is the ECS render-path overhead.
//
// Colliders stay out of both modes on purpose: the broad-phase cost would
// dominate the signal at high sprite counts and has its own benchmarks.
type Game struct {
	assets   asset.Manifest
	textures [textureCount]render.TextureID

	sprites      []spriteState
	spriteCount  int
	multiTexture bool

	useECS   bool
	entities []scene.EntityID

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
	g := &Game{assets: manifest}
	for i := range g.textures {
		g.textures[i] = manifest.Texture(fmt.Sprintf("sprite_%d", i))
	}
	return g, nil
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
		InputBindings: spritebenchBindings(),
	}
}

func (g *Game) Init(*runtime.UpdateContext) {
	g.restart()
}

func (g *Game) restart() {
	g.spriteCount = defaultSpriteCount
	g.multiTexture = false
	g.sprites = g.sprites[:0]
	g.growTo(g.spriteCount)
}

func (g *Game) Update(ctx *runtime.UpdateContext, dt float64) {
	if ctx.Input.JustPressed(input.ActionJump) {
		g.setSpriteCount(ctx.World, g.spriteCount*2)
	}
	if ctx.Input.JustPressed(input.ActionCancel) {
		g.setSpriteCount(ctx.World, g.spriteCount/2)
	}
	if ctx.Input.JustPressed(input.ActionConfirm) {
		g.multiTexture = !g.multiTexture
		if g.useECS {
			for i, id := range g.entities {
				ctx.World.SetImageSprite(id, g.imageSpriteFor(i))
			}
		}
	}
	if ctx.Input.JustPressed(input.ActionFocusNext) {
		g.useECS = !g.useECS
		if g.useECS {
			g.spawnEntities(ctx.World)
		} else {
			g.despawnEntities(ctx.World)
		}
	}

	for i := range g.sprites {
		s := &g.sprites[i]
		s.Pos.X += s.Vel.X * dt
		s.Pos.Y += s.Vel.Y * dt
		s.Rotation += s.Spin * dt

		if s.Pos.X < 0 {
			s.Pos.X = -s.Pos.X
			s.Vel.X = -s.Vel.X
		} else if limit := float64(worldWidth - spriteSize); s.Pos.X > limit {
			s.Pos.X = 2*limit - s.Pos.X
			s.Vel.X = -s.Vel.X
		}
		if s.Pos.Y < 0 {
			s.Pos.Y = -s.Pos.Y
			s.Vel.Y = -s.Vel.Y
		} else if limit := float64(worldHeight - spriteSize); s.Pos.Y > limit {
			s.Pos.Y = 2*limit - s.Pos.Y
			s.Vel.Y = -s.Vel.Y
		}
	}

	// The sprite slice stays the single source of motion in both modes; ECS
	// mode mirrors it into the World every tick, which is exactly the store
	// write traffic a transform-driven game generates.
	if g.useECS {
		for i, id := range g.entities {
			ctx.World.SetTransform(id, g.transformFor(i))
		}
	}
}

func (g *Game) setSpriteCount(w *scene.World, n int) {
	n = clampInt(n, minSpriteCount, maxSpriteCount)
	if n == g.spriteCount {
		return
	}
	if n > len(g.sprites) {
		g.growTo(n)
	} else {
		g.sprites = g.sprites[:n]
	}
	g.spriteCount = n
	if g.useECS {
		// Count changes are rare user events; a full respawn keeps the
		// entity list trivially in sync with the sprite slice.
		g.despawnEntities(w)
		g.spawnEntities(w)
	}
}

// transformFor converts a sprite's corner-anchored state to the ECS
// center-anchored transform so both modes draw identical rectangles.
func (g *Game) transformFor(i int) scene.Transform {
	s := &g.sprites[i]
	return scene.Transform{
		Position: math2d.Vec2{X: s.Pos.X + spriteSize/2, Y: s.Pos.Y + spriteSize/2},
		Rotation: s.Rotation,
		Scale:    math2d.Vec2{X: 1, Y: 1},
	}
}

func (g *Game) imageSpriteFor(i int) scene.ImageSprite {
	texture := g.textures[0]
	if g.multiTexture {
		texture = g.textures[i%textureCount]
	}
	return scene.ImageSprite{
		Texture: texture,
		Size:    math2d.Vec2{X: spriteSize, Y: spriteSize},
	}
}

func (g *Game) spawnEntities(w *scene.World) {
	g.entities = g.entities[:0]
	for i := range g.sprites {
		id := w.Spawn()
		w.SetTransform(id, g.transformFor(i))
		w.SetImageSprite(id, g.imageSpriteFor(i))
		g.entities = append(g.entities, id)
	}
}

func (g *Game) despawnEntities(w *scene.World) {
	// Despawn highest EntityID first: component stores keep their dense
	// arrays sorted by id, so removing from the front shifts and reindexes
	// the whole tail on every call — front-first mass despawn is O(n^2) and
	// visibly stalls at tens of thousands of entities. Tail-first is O(n).
	for i := len(g.entities) - 1; i >= 0; i-- {
		w.Despawn(g.entities[i])
	}
	g.entities = g.entities[:0]
}

func (g *Game) growTo(n int) {
	for i := len(g.sprites); i < n; i++ {
		g.sprites = append(g.sprites, spawnSprite(i))
	}
}

func (g *Game) Render(ctx *runtime.RenderContext) []render.Command {
	var commands []render.Command
	if g.useECS {
		commands = ctx.World.RenderCommands()
	} else {
		commands = make([]render.Command, 0, len(g.sprites))
		for i, s := range g.sprites {
			texture := g.textures[0]
			if g.multiTexture {
				texture = g.textures[i%textureCount]
			}
			commands = append(commands, render.DrawImage{
				Texture:  texture,
				Dest:     math2d.NewRect(s.Pos, math2d.Vec2{X: spriteSize, Y: spriteSize}),
				Rotation: s.Rotation,
			})
		}
	}

	hud := spritebenchLayout(ctx.Camera().Viewport)
	d := ctx.UI.In(ui.SpaceScreen)
	d.Panel(hud.panel)
	d.Label("SPRITE RENDER BENCHMARK", hud.title)
	d.Label(fmt.Sprintf("sprites  %s     textures  %s     path  %s", formatCount(len(g.sprites)), g.textureModeLabel(), g.pathModeLabel()), hud.load)
	d.Label(fmt.Sprintf("FPS  %5.1f     frame  %6.2f ms     P99  %6.2f ms", g.stats.fps, milliseconds(g.stats.frameTime), milliseconds(g.stats.p99)), hud.frame)
	d.Label(g.gpuLabel(), hud.gpu)
	d.Label(g.cpuEstimateLabel(), hud.cpu)
	d.Label("UP doubles, DOWN halves, T toggles 1/4 textures, E toggles direct/ECS", hud.help)
	return commands
}

// ObserveFrame receives host timing after rendering, mirroring the
// particlebench pattern.
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
) {
	g.stats = frameStats{
		fps: fps, frameTime: frameTime, p99: p99,
		gpuDraw: gpuDraw, hasGPUTime: hasGPUTime,
	}
}

func (g *Game) textureModeLabel() string {
	if g.multiTexture {
		return "4 (interleaved)"
	}
	return "1"
}

func (g *Game) pathModeLabel() string {
	if g.useECS {
		return "ECS"
	}
	return "direct"
}

// gpuLabel reports the main render pass's GPU time; the particle compute
// timings are always zero in this particle-free scene and stay hidden.
func (g *Game) gpuLabel() string {
	if !g.stats.hasGPUTime {
		return "GPU draw  unavailable (no timestamp-query)"
	}
	return fmt.Sprintf("GPU  draw  %6.2f ms", milliseconds(g.stats.gpuDraw))
}

// cpuEstimateLabel approximates the CPU-side cost of the tess/upload path:
// the render-loop wall time minus GPU draw time contains command building,
// tess.Build, the Bytes/TexBytes conversions and the bytesToJS heap copies.
// It is an estimate, not a measurement of any single stage.
func (g *Game) cpuEstimateLabel() string {
	if !g.stats.hasGPUTime {
		return "cpu ~= frame - gpu draw  unavailable (no gpu draw sample)"
	}
	cpu := g.stats.frameTime - g.stats.gpuDraw
	if cpu < 0 {
		cpu = 0
	}
	return fmt.Sprintf("cpu ~= frame - gpu draw  %6.2f ms", milliseconds(cpu))
}

func spritebenchBindings() []input.Binding {
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

type spritebenchHUD struct {
	panel                        math2d.Rect
	title, load, frame, gpu, cpu math2d.Vec2
	help                         math2d.Vec2
}

func spritebenchLayout(viewport math2d.Rect) spritebenchHUD {
	w, h := viewport.Width(), viewport.Height()
	panelW := math2d.Clamp(w*0.48, 520, 720)
	panelH := math2d.Clamp(h*0.36, 260, 320)
	x := viewport.Min.X + (w-panelW)/2
	y := viewport.Min.Y + math2d.Clamp(h*0.035, 18, 42)
	line := panelH * 0.10
	return spritebenchHUD{
		panel: math2d.NewRect(math2d.Vec2{X: x, Y: y}, math2d.Vec2{X: panelW, Y: panelH}),
		title: math2d.Vec2{X: x + panelW/2, Y: y + line*0.8},
		load:  math2d.Vec2{X: x + panelW/2, Y: y + line*1.9},
		frame: math2d.Vec2{X: x + panelW/2, Y: y + line*3.0},
		gpu:   math2d.Vec2{X: x + panelW/2, Y: y + line*4.1},
		cpu:   math2d.Vec2{X: x + panelW/2, Y: y + line*5.2},
		help:  math2d.Vec2{X: x + panelW/2, Y: y + line*6.6},
	}
}
