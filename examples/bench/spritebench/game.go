// Package spritebench is a browser performance benchmark for the WebGPU
// sprite render path: tess.Build packing render.DrawImage commands into
// storage records, followed by the per-frame JS-heap upload and instanced
// draw. It keeps its own sprite state instead of the ECS so results reflect
// only the tess/upload path, not scene.World bookkeeping.
package spritebench

import (
	"embed"
	"fmt"
	"io/fs"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/dhannyell/arasy/asset"
	"github.com/dhannyell/arasy/input"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
	"github.com/dhannyell/arasy/runtime"
	"github.com/dhannyell/arasy/scene"
	"github.com/dhannyell/arasy/ui"
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

	retainedBatchID = render.SpriteBatchID(1)

	// Culling scenario: cullingBatchCount retained batches of
	// cullingSpritesPerBatch sprites each. The first cullingVisibleBatches
	// scatter inside the camera's unmodified default World rect
	// (0,0)-(worldWidth,worldHeight); every other batch is offset far
	// outside it, so exactly cullingVisibleBatches survive whole-batch
	// culling without touching the camera at all.
	cullingVisibleBatches  = 10
	cullingBatchCount      = 100
	cullingSpritesPerBatch = 1000
	cullingBatchIDBase     = render.SpriteBatchID(1000)
	// cullingOffscreenStep places every culled batch on its own multiple of
	// worldWidth, far past any coordinate the default camera or its clip
	// could ever resolve to.
	cullingOffscreenStep = worldWidth * 4
)

// pathMode selects how Render builds sprite commands, isolating a different
// cost each time: direct command construction, ECS bookkeeping, retained
// whole-batch replace, or the whole-batch culling scenario (100 spatial
// batches, only cullingVisibleBatches in the camera).
type pathMode int

const (
	pathDirect pathMode = iota
	pathECS
	pathRetained
	pathCulling
)

func (p pathMode) next() pathMode { return (p + 1) % 4 }

// mutationPercents cycles the fraction of retained sprites that actually
// move each tick. v1 retained batches always replace whole-batch, so every
// step above 0% pays the same full re-upload cost — the point of the
// control is to make that visible, not to demonstrate a speedup.
var mutationPercents = []int{0, 1, 10, 100}

// Crowd animation constants for the retained animated-torcida stressor.
// Bounce/Phase are baked into each instance once at publish time (static
// content); BounceHz/Intensity ride the per-draw SpriteBatchAnimation
// uniform, so animating never touches the retained buffer or re-uploads
// content — only the small per-frame arena slot changes.
const (
	crowdBounceHz  = 1.1
	crowdIntensity = 1.0
	crowdBounce    = 6 // world units of per-sprite bounce amplitude
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

	cpuTick         time.Duration
	cpuRender       time.Duration
	hasCPUTime      bool
	cpuTess         time.Duration
	cpuUpload       time.Duration
	cpuEncode       time.Duration
	hasCPUBreakdown bool

	retainedGPUBytes     uint64
	retainedContentBytes uint64
	retainedTotal        int
	retainedVisible      int
	retainedDraws        int
	hasRetained          bool
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

	path     pathMode
	entities []scene.EntityID

	// mutationIndex selects mutationPercents; only meaningful in pathRetained.
	mutationIndex    int
	retainedDirty    bool // forces a republish (structural change or first frame)
	cullingPublished bool

	// animated toggles the retained "torcida" stressor: every instance
	// carries a baked-in bounce amplitude/phase, and Render drives the
	// batch-wide bounce uniform from animTime. Only meaningful in
	// pathRetained.
	animated bool
	animTime float64

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
	g.path = pathDirect
	g.mutationIndex = 0
	g.animated = false
	g.animTime = 0
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
		if g.path == pathECS {
			for i, id := range g.entities {
				ctx.World.SetImageSprite(id, g.imageSpriteFor(i))
			}
		}
		g.retainedDirty = true
	}
	if ctx.Input.JustPressed(input.ActionFocusNext) {
		g.setPath(ctx, g.path.next())
	}
	if ctx.Input.JustPressed(input.ActionPause) {
		g.mutationIndex = (g.mutationIndex + 1) % len(mutationPercents)
	}
	if ctx.Input.JustPressed(input.ActionMoveX) {
		// Toggling animation changes which instance records carry a
		// non-zero Animation (planSpriteBatch selects the animated pipeline
		// from that), so it needs one repack — after that, animating is
		// GPU-only: the batch buffer never changes again while animated
		// stays on.
		g.animated = !g.animated
		g.retainedDirty = true
	}
	g.animTime += dt

	if g.path == pathCulling {
		g.publishCullingBatchesOnce(ctx)
		return
	}

	// mutated bounds how many sprites actually move this tick. direct/ECS
	// always move every sprite; retained mode moves only the fraction
	// mutationPercents[g.mutationIndex] selects, so the HUD can show that
	// even a small mutated fraction still forces a full-batch re-upload —
	// v1 retained batches have no partial update.
	mutated := len(g.sprites)
	if g.path == pathRetained {
		mutated = g.spriteCount * mutationPercents[g.mutationIndex] / 100
	}
	for i := 0; i < mutated; i++ {
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

	// The sprite slice stays the single source of motion in ECS mode too; it
	// mirrors positions into the World every tick, which is exactly the
	// store write traffic a transform-driven game generates.
	if g.path == pathECS {
		for i, id := range g.entities {
			ctx.World.SetTransform(id, g.transformFor(i))
		}
	}
	if g.path == pathRetained && mutated > 0 {
		g.retainedDirty = true
	}
}

// setPath transitions between direct, ECS, retained and culling, doing the
// one-time setup/teardown each mode needs: ECS entities, the culling
// scenario's 100 retained batches, and the retained path's first publish.
func (g *Game) setPath(ctx *runtime.UpdateContext, next pathMode) {
	if next == g.path {
		return
	}
	if g.path == pathECS {
		g.despawnEntities(ctx.World)
	}
	if g.path == pathCulling && g.cullingPublished {
		g.deleteCullingBatches(ctx)
	}
	if g.path == pathRetained {
		ctx.Batches.DeleteSprites(retainedBatchID)
	}
	g.path = next
	switch next {
	case pathECS:
		g.spawnEntities(ctx.World)
	case pathRetained:
		g.retainedDirty = true
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
	if g.path == pathECS {
		// Count changes are rare user events; a full respawn keeps the
		// entity list trivially in sync with the sprite slice.
		g.despawnEntities(w)
		g.spawnEntities(w)
	}
	g.retainedDirty = true
}

// publishCullingBatchesOnce publishes cullingBatchCount retained batches
// once. The first cullingVisibleBatches scatter inside
// (0,0)-(worldWidth,worldHeight), the same World rect the default camera
// already shows; every other batch sits at its own multiple of
// cullingOffscreenStep on the X axis, far past anything that rect (or its
// clip) could ever intersect. No camera change is needed: whole-batch
// culling alone should keep exactly cullingVisibleBatches on screen.
func (g *Game) publishCullingBatchesOnce(ctx *runtime.UpdateContext) {
	if g.cullingPublished {
		return
	}
	for i := 0; i < cullingBatchCount; i++ {
		originX := 0.0
		if i >= cullingVisibleBatches {
			originX = float64(i) * cullingOffscreenStep
		}

		instances := make([]render.SpriteInstance, cullingSpritesPerBatch)
		for j := range instances {
			// i in the seed keeps the cullingVisibleBatches from scattering
			// identically on top of each other.
			pos := spawnSprite(j*cullingBatchCount + i).Pos
			instances[j] = render.SpriteInstance{
				Texture: g.textures[0],
				Dest: math2d.NewRect(
					math2d.Vec2{X: originX + pos.X, Y: pos.Y},
					math2d.Vec2{X: spriteSize, Y: spriteSize},
				),
			}
		}
		ctx.Batches.UpdateSprites(cullingBatchIDBase+render.SpriteBatchID(i), instances)
	}
	g.cullingPublished = true
}

func (g *Game) deleteCullingBatches(ctx *runtime.UpdateContext) {
	for i := 0; i < cullingBatchCount; i++ {
		ctx.Batches.DeleteSprites(cullingBatchIDBase + render.SpriteBatchID(i))
	}
	g.cullingPublished = false
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

func (g *Game) spriteTexture(i int) render.TextureID {
	if g.multiTexture {
		return g.textures[i%textureCount]
	}
	return g.textures[0]
}

func (g *Game) imageSpriteFor(i int) scene.ImageSprite {
	return scene.ImageSprite{
		Texture: g.spriteTexture(i),
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
	w.DespawnBatch(g.entities)
	g.entities = g.entities[:0]
}

func (g *Game) growTo(n int) {
	for i := len(g.sprites); i < n; i++ {
		g.sprites = append(g.sprites, spawnSprite(i))
	}
}

func (g *Game) Render(ctx *runtime.RenderContext) []render.Command {
	var commands []render.Command
	switch g.path {
	case pathECS:
		commands = ctx.World.RenderCommands()
	case pathRetained:
		if g.retainedDirty {
			ctx.Batches.UpdateSprites(retainedBatchID, g.retainedInstances())
			g.retainedDirty = false
		}
		batchCmd := render.DrawSpriteBatch{Batch: retainedBatchID}
		if g.animated {
			// Only Time changes frame to frame; BounceHz/Intensity are
			// constants. This is the one piece of "animated" state allowed
			// to vary every frame without touching the retained buffer —
			// it rides the draw command's small per-frame uniform slot.
			batchCmd.Animation = render.SpriteBatchAnimation{
				Time:      g.animTime,
				BounceHz:  crowdBounceHz,
				Intensity: crowdIntensity,
			}
		}
		commands = []render.Command{batchCmd}
	case pathCulling:
		commands = g.renderCullingCommands(ctx)
	default: // pathDirect
		commands = g.directCommands()
	}

	hud := spritebenchLayout(ctx.Camera().Viewport)
	d := ctx.UI.In(ui.SpaceScreen)
	d.Panel(hud.panel)
	d.Label("SPRITE RENDER BENCHMARK", hud.title)
	d.Label(fmt.Sprintf("sprites  %s     textures  %s     path  %s", formatCount(len(g.sprites)), g.textureModeLabel(), g.pathModeLabel()), hud.load)
	d.Label(fmt.Sprintf("FPS  %5.1f     frame  %6.2f ms     P99  %6.2f ms", g.stats.fps, milliseconds(g.stats.frameTime), milliseconds(g.stats.p99)), hud.frame)
	d.Label(g.gpuLabel(), hud.gpu)
	d.Label(g.cpuLabel(), hud.cpu)
	d.Label(g.cpuBreakdownLabel(), hud.breakdown)
	d.Label(g.retainedLabel(), hud.retained)
	d.Label("UP/DOWN count, T textures, E direct/ECS/retained/culling, M mutation %, A toggles crowd animation", hud.help)
	return commands
}

func (g *Game) directCommands() []render.Command {
	commands := make([]render.Command, 0, len(g.sprites))
	for i, s := range g.sprites {
		commands = append(commands, render.DrawImage{
			Texture:  g.spriteTexture(i),
			Dest:     math2d.NewRect(s.Pos, math2d.Vec2{X: spriteSize, Y: spriteSize}),
			Rotation: s.Rotation,
		})
	}
	return commands
}

func (g *Game) retainedInstances() []render.SpriteInstance {
	instances := make([]render.SpriteInstance, len(g.sprites))
	for i, s := range g.sprites {
		inst := render.SpriteInstance{
			Texture:  g.spriteTexture(i),
			Dest:     math2d.NewRect(s.Pos, math2d.Vec2{X: spriteSize, Y: spriteSize}),
			Rotation: s.Rotation,
		}
		if g.animated {
			// Phase is a pure function of index so the crowd bounces out of
			// sync instead of as one rigid block.
			inst.Animation = render.SpriteAnimation{
				Bounce: crowdBounce,
				Phase:  float32(math.Mod(float64(i)*0.129, 1)),
			}
		}
		instances[i] = inst
	}
	return instances
}

// renderCullingCommands leaves the camera untouched (see
// publishCullingBatchesOnce) and emits every batch command: culling which
// cullingVisibleBatches survive is the renderer's job, not the game's.
func (g *Game) renderCullingCommands(*runtime.RenderContext) []render.Command {
	commands := make([]render.Command, cullingBatchCount)
	for i := range commands {
		commands[i] = render.DrawSpriteBatch{Batch: cullingBatchIDBase + render.SpriteBatchID(i)}
	}
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
	cpuTick, cpuRender time.Duration,
	hasCPUTime bool,
	cpuTess, cpuUpload, cpuEncode time.Duration,
	hasCPUBreakdown bool,
	retainedGPUBytes, retainedContentBytes uint64,
	retainedTotal, retainedVisible, retainedDraws int,
	hasRetained bool,
) {
	g.stats = frameStats{
		fps: fps, frameTime: frameTime, p99: p99,
		gpuDraw: gpuDraw, hasGPUTime: hasGPUTime,
		cpuTick: cpuTick, cpuRender: cpuRender, hasCPUTime: hasCPUTime,
		cpuTess: cpuTess, cpuUpload: cpuUpload, cpuEncode: cpuEncode,
		hasCPUBreakdown:  hasCPUBreakdown,
		retainedGPUBytes: retainedGPUBytes, retainedContentBytes: retainedContentBytes,
		retainedTotal: retainedTotal, retainedVisible: retainedVisible, retainedDraws: retainedDraws,
		hasRetained: hasRetained,
	}
}

func (g *Game) textureModeLabel() string {
	if g.multiTexture {
		return "4 (interleaved)"
	}
	return "1"
}

func (g *Game) pathModeLabel() string {
	switch g.path {
	case pathECS:
		return "ECS"
	case pathRetained:
		if g.animated {
			return fmt.Sprintf("retained (mutate %d%%, animated)", mutationPercents[g.mutationIndex])
		}
		return fmt.Sprintf("retained (mutate %d%%)", mutationPercents[g.mutationIndex])
	case pathCulling:
		return fmt.Sprintf("culling (%d batches x %d)", cullingBatchCount, cullingSpritesPerBatch)
	default:
		return "direct"
	}
}

// retainedLabel reports GPU-resident bytes across every registered retained
// batch, content bytes actually uploaded this frame (0 at steady state),
// visible-vs-total instance count and the number of retained draw calls —
// WebGPU only, and only meaningful once a retained/culling path has
// published at least one batch.
func (g *Game) retainedLabel() string {
	if !g.stats.hasRetained {
		return "retained batches unavailable (Canvas2D)"
	}
	if g.path != pathRetained && g.path != pathCulling {
		return "retained  n/a in this path"
	}
	return fmt.Sprintf(
		"retained  gpu %s B  upload %s B/frame  visible %s/%s  draws %d",
		formatCount(int(g.stats.retainedGPUBytes)),
		formatCount(int(g.stats.retainedContentBytes)),
		formatCount(g.stats.retainedVisible),
		formatCount(g.stats.retainedTotal),
		g.stats.retainedDraws,
	)
}

// gpuLabel reports the main render pass's GPU time; the particle compute
// timings are always zero in this particle-free scene and stay hidden.
func (g *Game) gpuLabel() string {
	if !g.stats.hasGPUTime {
		return "GPU draw  unavailable (no timestamp-query)"
	}
	return fmt.Sprintf("GPU  draw  %6.2f ms", milliseconds(g.stats.gpuDraw))
}

// cpuLabel reports the measured Go-side wall time of the frame loop, split
// into the simulation tick and the render traversal (game command building +
// renderer). Falls back to the frame-minus-GPU estimate when the host does
// not report CPU timings.
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
// (or scene.World extracting render poses in ECS mode) plus dispatch/sort.
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

func spritebenchBindings() []input.Binding {
	return []input.Binding{
		{Source: input.Keyboard(input.KeyArrowUp), Action: input.ActionJump, Scale: 1},
		{Source: input.Keyboard(input.KeyArrowDown), Action: input.ActionCancel, Scale: 1},
		{Source: input.Keyboard(input.KeyT), Action: input.ActionConfirm, Scale: 1},
		{Source: input.Keyboard(input.KeyE), Action: input.ActionFocusNext, Scale: 1},
		{Source: input.Keyboard(input.KeyM), Action: input.ActionPause, Scale: 1},
		{Source: input.Keyboard(input.KeyA), Action: input.ActionMoveX, Scale: 1},
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
	breakdown, retained, help    math2d.Vec2
}

func spritebenchLayout(viewport math2d.Rect) spritebenchHUD {
	w, h := viewport.Width(), viewport.Height()
	panelW := math2d.Clamp(w*0.48, 560, 760)
	panelH := math2d.Clamp(h*0.44, 320, 380)
	x := viewport.Min.X + (w-panelW)/2
	y := viewport.Min.Y + math2d.Clamp(h*0.035, 18, 42)
	line := panelH * 0.08
	return spritebenchHUD{
		panel:     math2d.NewRect(math2d.Vec2{X: x, Y: y}, math2d.Vec2{X: panelW, Y: panelH}),
		title:     math2d.Vec2{X: x + panelW/2, Y: y + line*0.8},
		load:      math2d.Vec2{X: x + panelW/2, Y: y + line*1.9},
		frame:     math2d.Vec2{X: x + panelW/2, Y: y + line*3.0},
		gpu:       math2d.Vec2{X: x + panelW/2, Y: y + line*4.1},
		cpu:       math2d.Vec2{X: x + panelW/2, Y: y + line*5.2},
		breakdown: math2d.Vec2{X: x + panelW/2, Y: y + line*6.3},
		retained:  math2d.Vec2{X: x + panelW/2, Y: y + line*7.4},
		help:      math2d.Vec2{X: x + panelW/2, Y: y + line*8.6},
	}
}
