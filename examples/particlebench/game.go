// Package particlebench is a self-tuning browser performance benchmark for
// the WebGPU particle renderer. It intentionally uses simple untextured
// particles so its results focus on particle simulation and blend throughput.
package particlebench

import (
	"embed"
	"fmt"
	"io/fs"
	"strconv"
	"strings"
	"time"

	"github.com/dhannyell/frameforge/asset"
	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
	"github.com/dhannyell/frameforge/runtime"
	"github.com/dhannyell/frameforge/ui"
)

//go:embed assets
var embeddedAssets embed.FS

const (
	particlesPerEmitter = 65536
	maximumEmitters     = 256
	initialParticles    = 100
	tracerCapacity      = 2048
	sparkCapacity       = 512
	maximumBufferBytes  = 1 << 30
	targetFPS           = 59.0
	targetP99           = 20 * time.Millisecond
	settleReports       = 12 // 3 seconds at the host's 4 Hz telemetry cadence.
	failureReports      = 3
	warmupReports       = 24 // let initial WebGPU work leave the P99 window.
)

type phase uint8

const (
	phaseCalibrating phase = iota
	phaseLimited
	phaseMaximum
)

type frameStats struct {
	fps        float64
	frameTime  time.Duration
	p99        time.Duration
	inputLag   time.Duration
	hasInput   bool
	jsHeap     uint64
	jsHeapMax  uint64
	hasJSHeap  bool
	gpuBuffers uint64
	hasGPU     bool
}

// Game adds one full 65K emitter at a time after a stable observation period.
// It stops on sustained failure, preserving the last stable load in the HUD.
type Game struct {
	assets asset.Manifest
	effect render.ParticleEffectID
	tracer render.ParticleEffectID
	spark  render.ParticleEffectID

	targetParticles int
	activeEmitters  int
	warmupReports   int
	stableReports   int
	failedReports   int
	phase           phase
	stats           frameStats
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
		effect: manifest.Effect("benchmark_particles"),
		tracer: manifest.Effect("benchmark_tracers"),
		spark:  manifest.Effect("benchmark_sparks"),
	}, nil
}

func (g *Game) Setup() runtime.Config {
	theme := ui.DefaultTheme()
	theme.Font = g.assets.Font("default")

	return runtime.Config{
		Camera: render.Camera{
			World:    math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 100, Y: 100}),
			Viewport: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 1920, Y: 1080}),
			Aspect:   render.AspectExpand,
		},
		SimulationHz: 60,
		FPS:          60,
		Clear:        render.RGBA8(4, 9, 18, 255),
		Assets:       g.assets,
		UITheme:      theme,
	}
}

func (g *Game) Init(*runtime.UpdateContext) {
	g.restart()
}

func (g *Game) Update(ctx *runtime.UpdateContext, _ float64) {
	if ctx.UI.ClickedIn("particlebench_restart", benchmarkLayout(ctx.Camera().Viewport).restart, ui.SpaceScreen) {
		g.restart()
	}
}

func (g *Game) Render(ctx *runtime.RenderContext) []render.Command {
	visible := ctx.Camera().Resolve().World
	tracers, sparks := g.tracerTarget(), g.sparkTarget()
	tracerEmitters := emitterCountFor(tracers, tracerCapacity)
	sparkEmitters := emitterCountFor(sparks, sparkCapacity)
	commands := make([]render.Command, 0, g.activeEmitters+tracerEmitters+sparkEmitters+1)
	commands = append(commands, render.DrawRect{
		BaseCommand: render.BaseCommand{LayerValue: -10, Clip: visible},
		Rect:        visible,
		Fill:        render.RGBA8(4, 9, 18, 255),
	})

	emitScale := math2d.Vec2{X: visible.Width() / 100, Y: visible.Height() / 100}
	for i := 0; i < g.activeEmitters; i++ {
		target := g.emitterTarget(i)
		commands = append(commands, render.DrawParticles{
			BaseCommand: render.BaseCommand{LayerValue: 0, Clip: visible},
			Effect:      g.effect,
			Emitter:     render.EmitterID(i + 1),
			Pos:         visible.Center(),
			EmitScale:   emitScale,
			Intensity:   float64(target) / particlesPerEmitter,
			Time:        ctx.Time,
			Dt:          ctx.Dt,
			Seed:        uint64(0xbeef0000 + i),
		})
	}
	for i := range tracerEmitters {
		commands = append(commands, g.visualCommand(
			g.tracer, render.EmitterID(10_001+i), visualEmitterTarget(tracers, i, tracerCapacity), tracerCapacity,
			ctx, visible, emitScale, uint64(0x7ace+i),
		))
	}
	for i := range sparkEmitters {
		commands = append(commands, g.visualCommand(
			g.spark, render.EmitterID(20_001+i), visualEmitterTarget(sparks, i, sparkCapacity), sparkCapacity,
			ctx, visible, emitScale, uint64(0x5a9c+i),
		))
	}

	hud := benchmarkLayout(ctx.Camera().Viewport)
	ctx.UI.PanelIn(hud.panel, ui.SpaceScreen)
	ctx.UI.LabelIn("WEBGPU PARTICLE BENCHMARK", hud.title, ui.SpaceScreen)
	ctx.UI.LabelIn(fmt.Sprintf("target  %s particles  /  %s slots", formatCount(g.targetParticles), formatCount(g.allocatedSlots())), hud.load, ui.SpaceScreen)
	ctx.UI.LabelIn(fmt.Sprintf("visuals  %s tracers  +  %s sparks", formatCount(tracers), formatCount(sparks)), hud.visuals, ui.SpaceScreen)
	ctx.UI.LabelIn(fmt.Sprintf("FPS  %5.1f     frame  %5.2f ms     P99  %5.2f ms", g.stats.fps, milliseconds(g.stats.frameTime), milliseconds(g.stats.p99)), hud.frame, ui.SpaceScreen)
	ctx.UI.LabelIn(g.memoryLabel(), hud.memory, ui.SpaceScreen)
	ctx.UI.LabelIn(g.inputLagLabel(), hud.input, ui.SpaceScreen)
	ctx.UI.LabelIn(g.statusLabel(), hud.status, ui.SpaceScreen)
	ctx.UI.LabelIn("Move or click inside the canvas to sample input-to-submit latency.", hud.help, ui.SpaceScreen)
	ctx.UI.ButtonIn("particlebench_restart", "RESTART", hud.restart, ui.SpaceScreen)
	return commands
}

func (g *Game) visualCommand(
	effect render.ParticleEffectID,
	emitter render.EmitterID,
	target, capacity int,
	ctx *runtime.RenderContext,
	visible math2d.Rect,
	emitScale math2d.Vec2,
	seed uint64,
) render.DrawParticles {
	return render.DrawParticles{
		BaseCommand: render.BaseCommand{LayerValue: 1, Clip: visible},
		Effect:      effect,
		Emitter:     emitter,
		Pos:         visible.Center(),
		EmitScale:   emitScale,
		Intensity:   float64(target) / float64(capacity),
		Time:        ctx.Time,
		Dt:          ctx.Dt,
		Seed:        seed,
	}
}

// ObserveFrame receives host timing after rendering. It intentionally reacts
// only after several reports, so a single browser scheduling hiccup cannot end
// the benchmark.
func (g *Game) ObserveFrame(
	fps float64,
	frameTime, p99, inputLag time.Duration,
	hasInput bool,
	jsHeap, jsHeapMax uint64,
	hasJSHeap bool,
	gpuBuffers uint64,
	hasGPU bool,
) {
	g.stats = frameStats{
		fps: fps, frameTime: frameTime, p99: p99, inputLag: inputLag, hasInput: hasInput,
		jsHeap: jsHeap, jsHeapMax: jsHeapMax, hasJSHeap: hasJSHeap,
		gpuBuffers: gpuBuffers, hasGPU: hasGPU,
	}
	if g.phase != phaseCalibrating {
		return
	}
	if g.warmupReports < warmupReports {
		g.warmupReports++
		return
	}
	if fps >= targetFPS && p99 <= targetP99 {
		g.stableReports++
		g.failedReports = 0
		if g.stableReports >= settleReports {
			if g.targetParticles >= maximumParticles() {
				g.phase = phaseMaximum
				return
			}
			g.targetParticles = nextParticleTarget(g.targetParticles)
			g.activeEmitters = emitterCount(g.targetParticles)
			g.stableReports = 0
		}
		return
	}

	g.stableReports = 0
	g.failedReports++
	if g.failedReports >= failureReports {
		g.phase = phaseLimited
	}
}

func (g *Game) restart() {
	g.targetParticles = initialParticles
	g.activeEmitters = emitterCount(g.targetParticles)
	g.warmupReports = 0
	g.stableReports = 0
	g.failedReports = 0
	g.phase = phaseCalibrating
}

func (g *Game) emitterTarget(index int) int {
	remaining := g.targetParticles - index*particlesPerEmitter
	return min(max(remaining, 0), particlesPerEmitter)
}

// tracerTarget is one-for-one through the first 2K particles, then retains a
// visible one-in-128 sample for the rest of the benchmark.
func (g *Game) tracerTarget() int {
	return tracerTargetFor(g.targetParticles)
}

func tracerTargetFor(target int) int {
	if target <= tracerCapacity {
		return target
	}
	return tracerCapacity + (target-tracerCapacity)/128
}

// sparkTarget starts at half the target, reaches a full spark pool at 1K, and
// then retains a visible one-in-512 sample as the workload keeps growing.
func (g *Game) sparkTarget() int {
	return sparkTargetFor(g.targetParticles)
}

func sparkTargetFor(target int) int {
	if target <= sparkCapacity*2 {
		return max(1, target/2)
	}
	return sparkCapacity + (target-sparkCapacity*2)/512
}

func (g *Game) allocatedSlots() int { return g.activeEmitters * particlesPerEmitter }

func emitterCount(target int) int {
	return emitterCountFor(target, particlesPerEmitter)
}

func emitterCountFor(target, capacity int) int {
	return max(1, (target+capacity-1)/capacity)
}

func visualEmitterTarget(total, index, capacity int) int {
	return min(max(total-index*capacity, 0), capacity)
}

func maximumParticles() int { return maximumEmitters * particlesPerEmitter }

// maximumParticleBufferBytes mirrors the per-emitter WebGPU allocation:
// particle state, three index lists, counters, indirect args and uniforms.
func maximumParticleBufferBytes() int {
	return maximumEmitters*particleBufferBytes(particlesPerEmitter) +
		emitterCountFor(tracerTargetFor(maximumParticles()), tracerCapacity)*particleBufferBytes(tracerCapacity) +
		emitterCountFor(sparkTargetFor(maximumParticles()), sparkCapacity)*particleBufferBytes(sparkCapacity)
}

func particleBufferBytes(capacity int) int {
	return capacity*(48+3*4) + 224 + 12 + 16 + 16
}

func nextParticleTarget(current int) int {
	if current >= maximumParticles() {
		return maximumParticles()
	}
	if current > maximumParticles()/2 {
		return maximumParticles()
	}
	return current * 2
}

func (g *Game) inputLagLabel() string {
	if !g.stats.hasInput {
		return "input to submit  --  (awaiting input)"
	}
	return fmt.Sprintf("input to submit  %.2f ms", milliseconds(g.stats.inputLag))
}

func (g *Game) memoryLabel() string {
	ram := "RAM JS  unavailable"
	if g.stats.hasJSHeap {
		ram = fmt.Sprintf("RAM JS  %s / %s", formatBytes(g.stats.jsHeap), formatBytes(g.stats.jsHeapMax))
	}
	gpu := "GPU buffers  unavailable"
	if g.stats.hasGPU {
		gpu = "GPU buffers est.  " + formatBytes(g.stats.gpuBuffers)
	}
	return ram + "     " + gpu
}

func (g *Game) statusLabel() string {
	switch g.phase {
	case phaseLimited:
		return "status  LIMIT REACHED — last stable load retained"
	case phaseMaximum:
		return "status  SAFETY CEILING REACHED — 1 GiB particle buffers"
	default:
		if g.warmupReports < warmupReports {
			return fmt.Sprintf("status  warming WebGPU — %.1f s", float64(warmupReports-g.warmupReports)/4)
		}
		return fmt.Sprintf("status  calibrating — %s next in %.1f s", formatCount(nextParticleTarget(g.targetParticles)), float64(settleReports-g.stableReports)/4)
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

func formatBytes(bytes uint64) string {
	const mib = 1024 * 1024
	if bytes < mib {
		return fmt.Sprintf("%d KiB", bytes/1024)
	}
	return fmt.Sprintf("%.1f MiB", float64(bytes)/mib)
}

type benchmarkHUD struct {
	panel                                              math2d.Rect
	title, load, visuals, frame, memory, input, status math2d.Vec2
	help                                               math2d.Vec2
	restart                                            math2d.Rect
}

func benchmarkLayout(viewport math2d.Rect) benchmarkHUD {
	w, h := viewport.Width(), viewport.Height()
	panelW := math2d.Clamp(w*0.48, 520, 720)
	panelH := math2d.Clamp(h*0.47, 340, 400)
	x := viewport.Min.X + (w-panelW)/2
	y := viewport.Min.Y + math2d.Clamp(h*0.035, 18, 42)
	line := panelH * 0.080
	return benchmarkHUD{
		panel:   math2d.NewRect(math2d.Vec2{X: x, Y: y}, math2d.Vec2{X: panelW, Y: panelH}),
		title:   math2d.Vec2{X: x + panelW/2, Y: y + line*0.65},
		load:    math2d.Vec2{X: x + panelW/2, Y: y + line*1.55},
		visuals: math2d.Vec2{X: x + panelW/2, Y: y + line*2.45},
		frame:   math2d.Vec2{X: x + panelW/2, Y: y + line*3.35},
		memory:  math2d.Vec2{X: x + panelW/2, Y: y + line*4.25},
		input:   math2d.Vec2{X: x + panelW/2, Y: y + line*5.15},
		status:  math2d.Vec2{X: x + panelW/2, Y: y + line*6.05},
		help:    math2d.Vec2{X: x + panelW/2, Y: y + panelH*0.72},
		restart: math2d.NewRect(
			math2d.Vec2{X: x + panelW*0.37, Y: y + panelH*0.86},
			math2d.Vec2{X: panelW * 0.26, Y: panelH * 0.09},
		),
	}
}
