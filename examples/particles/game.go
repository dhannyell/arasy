// Package particles is an example of engine particles system
package particles

import (
	"embed"
	"fmt"
	"io/fs"
	"math"

	"github.com/dhannyell/arasy/asset"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
	"github.com/dhannyell/arasy/runtime"
	"github.com/dhannyell/arasy/ui"
)

//go:embed assets
var embeddedAssets embed.FS

const (
	defaultDensity         = 0.5
	targetParticleCapacity = 65536
)

var densitySliderOptions = ui.SliderOptions{Min: 0, Max: 4, Step: 0.05}

type season uint8

const (
	seasonSpring season = iota
	seasonSummer
	seasonAutumn
	seasonWinter
)

type viewportAnchor uint8

const (
	anchorNone viewportAnchor = iota
	anchorTopLeft
	anchorTopRight
	anchorBottomLeft
	anchorBottomRight
)

type particleLayer struct {
	effect        string
	emitter       render.EmitterID
	seed          uint64
	position      math2d.Vec2
	windPhase     float64
	windStrength  float64
	renderLayer   int
	scaleViewport bool
	anchor        viewportAnchor
}

var springLayers = [...]particleLayer{
	{effect: "spring_petals_broad", emitter: 1, seed: 0x51a7, position: math2d.Vec2{X: 50, Y: -2}, windPhase: 0.3, windStrength: 1, scaleViewport: true},
	{effect: "spring_petals_curved", emitter: 2, seed: 0xc4e1, position: math2d.Vec2{X: 50, Y: -2}, windPhase: 2.1, windStrength: 0.85, scaleViewport: true},
	{effect: "spring_petals_folded", emitter: 3, seed: 0xf01d, position: math2d.Vec2{X: 50, Y: -2}, windPhase: 4.7, windStrength: 1.15, scaleViewport: true},
	{effect: "spring_leaves", emitter: 4, seed: 0x1eaf, position: math2d.Vec2{X: 50, Y: -2}, windPhase: 1.4, windStrength: 1.35, scaleViewport: true},
	{effect: "spring_pollen", emitter: 5, seed: 0xb011, position: math2d.Vec2{X: 50, Y: 50}, windPhase: 3.2, windStrength: 0.2, scaleViewport: true},
	{effect: "spring_butterflies", emitter: 6, seed: 0xb077, position: math2d.Vec2{X: -4, Y: 50}, windPhase: 5.4, windStrength: 0.8, scaleViewport: true},
	{effect: "spring_dandelion_seeds", emitter: 7, seed: 0xda7d, position: math2d.Vec2{X: 50, Y: 50}, windPhase: 0.9, windStrength: 0.45, scaleViewport: true},
}

var summerLayers = [...]particleLayer{
	{effect: "summer_glow", emitter: 100, seed: 0x6107, position: math2d.Vec2{X: 50, Y: 72}, windPhase: 0.8, windStrength: 0.04, renderLayer: -2},
	{effect: "summer_heat_haze", emitter: 105, seed: 0x4ea7, position: math2d.Vec2{X: 50, Y: 75}, windPhase: 3.6, windStrength: 0.04, renderLayer: -1, scaleViewport: true},
	{effect: "summer_smoke", emitter: 101, seed: 0x5a0e, position: math2d.Vec2{X: 50, Y: 68}, windPhase: 4.1, windStrength: 0.35, renderLayer: -1},
	{effect: "summer_flame_outer", emitter: 102, seed: 0xf107, position: math2d.Vec2{X: 50, Y: 70}, windPhase: 2.2, windStrength: 0.18},
	{effect: "summer_golden_dust", emitter: 106, seed: 0xd057, position: math2d.Vec2{X: 50, Y: 50}, windPhase: 2.8, windStrength: 0.08, renderLayer: 1, scaleViewport: true},
	{effect: "summer_flame_core", emitter: 103, seed: 0xf1ae, position: math2d.Vec2{X: 50, Y: 70}, windPhase: 0.4, windStrength: 0.12, renderLayer: 1},
	{effect: "summer_embers", emitter: 104, seed: 0xe8be, position: math2d.Vec2{X: 50, Y: 69}, windPhase: 1.7, windStrength: 0.3, renderLayer: 2},
	{effect: "summer_fireflies", emitter: 107, seed: 0xf1ef, position: math2d.Vec2{X: 50, Y: 52}, windPhase: 5.2, windStrength: 0.5, renderLayer: 2, scaleViewport: true},
	{effect: "summer_ash_hot", emitter: 108, seed: 0xa517, position: math2d.Vec2{X: 50, Y: 66}, windPhase: 0.6, windStrength: 0.35, renderLayer: 3},
	{effect: "summer_ash_cold", emitter: 109, seed: 0xc01d, position: math2d.Vec2{X: 50, Y: -4}, windPhase: 3.8, windStrength: 0.65, renderLayer: 3, scaleViewport: true},
}

var autumnLayers = [...]particleLayer{
	{effect: "autumn_ground_fog", emitter: 200, seed: 0xf067, position: math2d.Vec2{X: 50, Y: 94}, windPhase: 1.1, windStrength: 0.08, renderLayer: -1, scaleViewport: true},
	{effect: "autumn_maple_red", emitter: 201, seed: 0xa11e, position: math2d.Vec2{X: 50, Y: -3}, windPhase: 0.5, windStrength: 1.15, scaleViewport: true},
	{effect: "autumn_maple_orange", emitter: 202, seed: 0x0a61, position: math2d.Vec2{X: 50, Y: -3}, windPhase: 3.3, windStrength: 0.9, scaleViewport: true},
	{effect: "autumn_samaras", emitter: 203, seed: 0x5eed, position: math2d.Vec2{X: 50, Y: -4}, windPhase: 4.7, windStrength: 0.55, renderLayer: 1, scaleViewport: true},
	{effect: "autumn_drizzle", emitter: 204, seed: 0xd122, position: math2d.Vec2{X: 50, Y: -5}, windPhase: 2.4, windStrength: 0.18, renderLayer: 2, scaleViewport: true},
}

var winterLayers = [...]particleLayer{
	{effect: "winter_snow_dust", emitter: 300, seed: 0x5d057, position: math2d.Vec2{X: 50, Y: -3}, windPhase: 0.7, windStrength: 0.18, renderLayer: -1, scaleViewport: true},
	{effect: "winter_condensation_mist", emitter: 301, seed: 0xc01d5, position: math2d.Vec2{X: 50, Y: 50}, windPhase: 2.3, windStrength: 0.08, scaleViewport: true},
	{effect: "winter_condensation_nuclei", emitter: 307, seed: 0x1ce01, position: math2d.Vec2{X: 50, Y: 50}, windPhase: 3.1, windStrength: 0.04, renderLayer: 2, scaleViewport: true},
	{effect: "winter_condensation_crystals", emitter: 308, seed: 0xc2a11, position: math2d.Vec2{X: 50, Y: 50}, windPhase: 4.6, windStrength: 0.08, renderLayer: 3, scaleViewport: true},
	{effect: "winter_snow_crystals", emitter: 302, seed: 0xc2a57, position: math2d.Vec2{X: 50, Y: -4}, windPhase: 4.1, windStrength: 0.55, renderLayer: 2, scaleViewport: true},
}

type demoState struct {
	Density float64
	Season  season
}

type Game struct {
	runtime.StateSlot[demoState]
	assets asset.Manifest
}

func (g *Game) Setup() runtime.Config {
	return runtime.Config{
		Camera: render.Camera{
			World:    math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 100, Y: 100}),
			Viewport: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 1920, Y: 1080}),
			Aspect:   render.AspectExpand,
		},
		SimulationHz: 60,
		FPS:          60,
		Assets:       g.assets,
	}
}

func (g *Game) Init(ctx *runtime.UpdateContext) {
	g.State.Density = defaultDensity
	g.State.Season = seasonSpring
}

func (g *Game) Update(ctx *runtime.UpdateContext, dt float64) {
	hud := particleHUDLayout(ctx.Camera().Viewport)
	in := ctx.UI.In(ui.SpaceScreen)
	if in.Clicked("season_spring", hud.springButton) {
		g.State.Season = seasonSpring
	}
	if in.Clicked("season_summer", hud.summerButton) {
		g.State.Season = seasonSummer
	}
	if in.Clicked("season_autumn", hud.autumnButton) {
		g.State.Season = seasonAutumn
	}
	if in.Clicked("season_winter", hud.winterButton) {
		g.State.Season = seasonWinter
	}

	value, changed := in.SliderInput(
		"particle_density",
		hud.densitySlider,
		g.State.Density,
		densitySliderOptions,
	)
	if changed {
		g.State.Density = value
	}
}

func (g *Game) Render(ctx *runtime.RenderContext) []render.Command {
	hud := particleHUDLayout(ctx.Camera().Viewport)
	density := g.State.Density
	title := "SPRING  /  SAKURA GARDEN"
	densityLabel := "Spring"
	background := "spring_background"
	overlay := render.RGBA8(3, 10, 16, 72)
	layers := springLayers[:]
	springLabel, summerLabel, autumnLabel, winterLabel := "[ SPRING ]", "SUMMER", "AUTUMN", "WINTER"

	if g.State.Season == seasonSummer {
		title = "SUMMER  /  EMBER MEADOW"
		densityLabel = "Summer"
		background = "summer_background"
		overlay = render.RGBA8(6, 10, 20, 90)
		layers = summerLayers[:]
		springLabel, summerLabel = "SPRING", "[ SUMMER ]"
	}
	if g.State.Season == seasonAutumn {
		title = "AUTUMN  /  MAPLE DRIFT"
		densityLabel = "Autumn"
		background = "autumn_background"
		overlay = render.RGBA8(12, 8, 10, 76)
		layers = autumnLayers[:]
		springLabel, summerLabel, autumnLabel = "SPRING", "SUMMER", "[ AUTUMN ]"
	}
	if g.State.Season == seasonWinter {
		title = "WINTER  /  FROSTED SILENCE"
		densityLabel = "Winter"
		background = "winter_background"
		overlay = render.RGBA8(5, 12, 22, 62)
		layers = winterLayers[:]
		springLabel, summerLabel, autumnLabel, winterLabel = "SPRING", "SUMMER", "AUTUMN", "[ WINTER ]"
	}

	cmds := make([]render.Command, 0, len(layers)+2)

	d := ctx.UI.In(ui.SpaceScreen)
	d.Panel(hud.titlePanel)
	d.Label(title, hud.titleCenter)
	d.Button("season_spring", springLabel, hud.springButton)
	d.Button("season_summer", summerLabel, hud.summerButton)
	d.Button("season_autumn", autumnLabel, hud.autumnButton)
	d.Button("season_winter", winterLabel, hud.winterButton)
	d.Panel(hud.densityPanel)
	d.Label(
		fmt.Sprintf("%s density  %.2fx", densityLabel, density),
		hud.densityLabel,
	)
	d.Label(
		fmt.Sprintf(
			"particles  %s / %s",
			formatParticleCount(activeParticleBudget(density)),
			formatParticleCount(targetParticleCapacity),
		),
		hud.particleCountLabel,
	)
	d.Slider(
		"particle_density",
		hud.densitySlider,
		density,
		densitySliderOptions,
	)
	visibleWorld := ctx.Camera().Resolve().World

	cmds = append(cmds, render.DrawImage{
		BaseCommand: render.BaseCommand{LayerValue: -10, Clip: visibleWorld},
		Texture:     ctx.Assets.Texture(background),
		Src:         backgroundCoverSource(ctx.Camera().Viewport),
		Dest:        visibleWorld,
	})

	cmds = append(cmds, render.DrawRect{
		BaseCommand: render.BaseCommand{LayerValue: -9, Clip: visibleWorld},
		Rect:        visibleWorld,
		Fill:        overlay,
	})
	for _, layer := range layers {
		cmds = append(cmds, particleCommand(ctx, layer, density, visibleWorld))
	}

	return cmds
}

func particleCommand(ctx *runtime.RenderContext, layer particleLayer, density float64, clip math2d.Rect) render.DrawParticles {
	position, emitScale := particleEmissionTransform(layer, clip)
	intensity := particleIntensity(density)
	if layer.effect == "summer_flame_core" || layer.effect == "summer_flame_outer" {
		position, emitScale = summerFlameTransform(position, density)
		intensity = summerFlameIntensity(density)
	}
	if layer.effect == "summer_ash_hot" || layer.effect == "summer_ash_cold" {
		intensity = summerAshIntensity(density)
	}
	if isWinterCondensation(layer.effect) {
		intensity = winterCondensationIntensity(density) * winterCondensationPulse(layer.effect, ctx.Time)
	}
	if layer.effect == "autumn_drizzle" {
		intensity *= 10.0
	}
	return render.DrawParticles{
		BaseCommand: render.BaseCommand{LayerValue: layer.renderLayer, Clip: clip},
		Effect:      ctx.Assets.Effect(layer.effect),
		Emitter:     layer.emitter,
		Pos:         position,
		EmitScale:   emitScale,
		Dir:         springWindOffset(ctx.Time, layer),
		Intensity:   intensity,
		Time:        ctx.Time,
		Dt:          ctx.Dt,
		Seed:        layer.seed,
	}
}

// particleIntensity uses a quadratic response so the low half of the slider
// remains sparse while the maximum still reaches the scene's full 65K budget.
func particleIntensity(density float64) float64 {
	progress := math2d.Clamp(density/densitySliderOptions.Max, 0, 1)
	return densitySliderOptions.Max * progress * progress
}

func activeParticleBudget(density float64) int {
	progress := math2d.Clamp(density/densitySliderOptions.Max, 0, 1)
	return int(math.Round(targetParticleCapacity * progress * progress))
}

func formatParticleCount(count int) string {
	if count < 1000 {
		return fmt.Sprintf("%d", count)
	}
	return fmt.Sprintf("%d.%03d", count/1000, count%1000)
}

func particleEmissionTransform(layer particleLayer, visible math2d.Rect) (math2d.Vec2, math2d.Vec2) {
	position := layer.position
	emitScale := math2d.Vec2{}
	if layer.scaleViewport {
		size := visible.Size()
		position = math2d.Vec2{
			X: visible.Min.X + size.X*layer.position.X/100,
			Y: visible.Min.Y + size.Y*layer.position.Y/100,
		}
		emitScale = math2d.Vec2{X: size.X / 100, Y: size.Y / 100}
	}
	return anchoredParticlePosition(position, visible, layer.anchor), emitScale
}

func anchoredParticlePosition(fallback math2d.Vec2, visible math2d.Rect, anchor viewportAnchor) math2d.Vec2 {
	const inset = 2.5
	switch anchor {
	case anchorTopLeft:
		return visible.Min.Add(math2d.Vec2{X: inset, Y: inset})
	case anchorTopRight:
		return math2d.Vec2{X: visible.Max.X - inset, Y: visible.Min.Y + inset}
	case anchorBottomLeft:
		return math2d.Vec2{X: visible.Min.X + inset, Y: visible.Max.Y - inset}
	case anchorBottomRight:
		return visible.Max.Sub(math2d.Vec2{X: inset, Y: inset})
	default:
		return fallback
	}
}

func isWinterCondensation(effect string) bool {
	return effect == "winter_condensation_mist" ||
		effect == "winter_condensation_nuclei" ||
		effect == "winter_condensation_crystals"
}

func winterCondensationIntensity(density float64) float64 {
	progress := math2d.Clamp(density/densitySliderOptions.Max, 0, 1)
	return math.Sqrt(progress)
}

func winterCondensationPulse(effect string, t float64) float64 {
	phase := math.Mod(t, 5)
	switch effect {
	case "winter_condensation_mist":
		if phase < 0.55 {
			return 1
		}
	case "winter_condensation_nuclei":
		if phase >= 0.25 && phase < 0.75 {
			return 1
		}
	case "winter_condensation_crystals":
		if phase >= 0.5 && phase < 1.05 {
			return 1
		}
	}
	return 0
}

// summerFlameTransform grows the spawn region without animating its origin.
// Keeping the core and outer emitters concentric prevents a dense fire from
// separating into orbiting blobs.
func summerFlameTransform(origin math2d.Vec2, density float64) (math2d.Vec2, math2d.Vec2) {
	growth := math2d.Clamp(density/densitySliderOptions.Max, 0, 1)
	emitScale := math2d.Vec2{
		X: math2d.Lerp(0.9, 1.45, growth),
		Y: math2d.Lerp(0.8, 1.8, growth),
	}
	return origin, emitScale
}

// summerFlameIntensity avoids additive saturation at the top of the scene's
// density range. The flame grows through its emission shape instead of piling
// thousands of identical sprites over the same point.
func summerFlameIntensity(density float64) float64 {
	progress := math2d.Clamp(density/densitySliderOptions.Max, 0, 1)
	return math.Sqrt(progress)
}

// summerAshIntensity stays visible at the demo's sparse default without
// flooding the flame at maximum density. Ash is an atmospheric accent, not a
// direct measure of the scene's overall particle budget.
func summerAshIntensity(density float64) float64 {
	progress := math2d.Clamp(density/densitySliderOptions.Max, 0, 1)
	return math.Sqrt(progress)
}

// springWindOffset combines non-harmonic gusts so each layer receives a
// distinct, slowly changing airflow without consuming simulation RNG.
func springWindOffset(t float64, layer particleLayer) float64 {
	slowDrift := 0.17 * math.Sin(t*0.23+layer.windPhase)
	crosswind := 0.10 * math.Sin(t*0.71+layer.windPhase*1.7)
	shortGust := 0.06 * math.Sin(t*1.93+layer.windPhase*0.43)
	return (slowDrift + crosswind + shortGust) * layer.windStrength
}

type particleHUD struct {
	titlePanel         math2d.Rect
	titleCenter        math2d.Vec2
	springButton       math2d.Rect
	summerButton       math2d.Rect
	autumnButton       math2d.Rect
	winterButton       math2d.Rect
	densityPanel       math2d.Rect
	densityLabel       math2d.Vec2
	particleCountLabel math2d.Vec2
	densitySlider      math2d.Rect
}

func particleHUDLayout(viewport math2d.Rect) particleHUD {
	w, h := viewport.Width(), viewport.Height()
	margin := math2d.Clamp(math.Min(w, h)*0.012, 6, 16)
	gap := math2d.Clamp(h*0.008, 4, 8)
	availableW := math.Max(1, w-2*margin)
	titleW := math.Min(availableW, math.Max(480, w*0.48))
	titleH := math2d.Clamp(h*0.06, 44, 60)
	buttonH := math2d.Clamp(h*0.045, 36, 46)
	buttonGap := math2d.Clamp(titleW*0.02, 6, 12)
	buttonW := (titleW - 3*buttonGap) / 4
	titleX := viewport.Min.X + (w-titleW)/2
	titleY := viewport.Min.Y + margin
	buttonY := titleY + titleH + gap

	densityW := math.Min(availableW, math.Max(340, w*0.38))
	densityH := math2d.Clamp(h*0.11, 90, 112)
	densityX := viewport.Min.X + (w-densityW)/2
	densityY := viewport.Max.Y - margin - densityH
	sliderInset := math.Min(40, densityW*0.08)
	sliderH := math2d.Clamp(densityH*0.23, 22, 28)

	return particleHUD{
		titlePanel: math2d.NewRect(
			math2d.Vec2{X: titleX, Y: titleY},
			math2d.Vec2{X: titleW, Y: titleH},
		),
		titleCenter: math2d.Vec2{X: viewport.Min.X + w/2, Y: titleY + titleH/2},
		springButton: math2d.NewRect(
			math2d.Vec2{X: titleX, Y: buttonY},
			math2d.Vec2{X: buttonW, Y: buttonH},
		),
		summerButton: math2d.NewRect(
			math2d.Vec2{X: titleX + buttonW + buttonGap, Y: buttonY},
			math2d.Vec2{X: buttonW, Y: buttonH},
		),
		autumnButton: math2d.NewRect(
			math2d.Vec2{X: titleX + 2*(buttonW+buttonGap), Y: buttonY},
			math2d.Vec2{X: buttonW, Y: buttonH},
		),
		winterButton: math2d.NewRect(
			math2d.Vec2{X: titleX + 3*(buttonW+buttonGap), Y: buttonY},
			math2d.Vec2{X: buttonW, Y: buttonH},
		),
		densityPanel: math2d.NewRect(
			math2d.Vec2{X: densityX, Y: densityY},
			math2d.Vec2{X: densityW, Y: densityH},
		),
		densityLabel:       math2d.Vec2{X: viewport.Min.X + w/2, Y: densityY + densityH*0.25},
		particleCountLabel: math2d.Vec2{X: viewport.Min.X + w/2, Y: densityY + densityH*0.47},
		densitySlider: math2d.NewRect(
			math2d.Vec2{X: densityX + sliderInset, Y: densityY + densityH*0.68},
			math2d.Vec2{X: densityW - 2*sliderInset, Y: sliderH},
		),
	}
}

func backgroundCoverSource(viewport math2d.Rect) math2d.Rect {
	const textureSize = 1024.0
	if viewport.Width() <= 0 || viewport.Height() <= 0 {
		return math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: textureSize, Y: textureSize})
	}
	aspect := viewport.Width() / viewport.Height()
	if aspect >= 1 {
		height := textureSize / aspect
		return math2d.NewRect(
			math2d.Vec2{X: 0, Y: (textureSize - height) / 2},
			math2d.Vec2{X: textureSize, Y: height},
		)
	}
	width := textureSize * aspect
	return math2d.NewRect(
		math2d.Vec2{X: (textureSize - width) / 2, Y: 0},
		math2d.Vec2{X: width, Y: textureSize},
	)
}

func NewGame() (*Game, error) {
	sub, err := fs.Sub(embeddedAssets, "assets")
	if err != nil {
		return nil, err
	}
	m, err := asset.Load(sub)
	if err != nil {
		return nil, err
	}
	return &Game{assets: m}, nil
}
