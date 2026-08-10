// Package postfx is an interactive showcase for Arasy's fullscreen effects.
package postfx

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
	"github.com/dhannyell/arasy/runtime"
	"github.com/dhannyell/arasy/ui"
)

const (
	viewW = 1280
	viewH = 720
)

type effectID uint8

const (
	effectVignette effectID = iota
	effectColorGrade
	effectPixelate
	effectChromatic
	effectBloom
	effectShockwave
	effectLensRain
	effectCount
)

var effectLabels = [...]string{
	"Vignette",
	"Color grade",
	"Pixelate",
	"Chromatic aberration",
	"Bloom",
	"Shockwave",
	"Lens rain",
}

var effectMenuLabels = [...]string{
	"Vignette",
	"Color grade",
	"Pixelate",
	"Chromatic",
	"Bloom",
	"Shockwave",
	"Lens rain",
}

type demoState struct {
	Enabled        [effectCount]bool
	Selected       effectID
	ShowParameters bool

	VignetteColor    render.Color
	VignetteStrength float64
	VignetteSoftness float64

	ColorGradeBrightness float64
	ColorGradeContrast   float64
	ColorGradeSaturation float64
	ColorGradeTint       render.Color

	PixelateSize       float64
	ChromaticStrength  float64
	BloomThreshold     float64
	BloomIntensity     float64
	BloomPasses        int
	ShockwaveCenter    math2d.Vec2
	ShockwaveRadius    float64
	ShockwaveWidth     float64
	ShockwaveStrength  float64
	LensRainDensity    float64
	LensRainSize       float64
	LensRainSpeed      float64
	LensRainRefraction float64
	LensRainSeed       uint64
	LensRainSeedText   string
}

// Game is the interactive post-processing showcase.
type Game struct {
	runtime.StateSlot[demoState]
}

// NewGame creates the post-processing showcase without external assets.
func NewGame() (*Game, error) {
	return &Game{}, nil
}

// Setup configures a fixed reference canvas so the scene and controls keep
// the same layout across native, raster, and browser hosts.
func (g *Game) Setup() runtime.Config {
	world := math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: viewW, Y: viewH})
	return runtime.Config{
		Camera:       render.Camera{World: world, Viewport: world, Aspect: render.AspectFit},
		Clear:        render.RGBA8(8, 12, 22, 255),
		Bounds:       world,
		SimulationHz: 60,
		FPS:          60,
		HistoryLen:   240,
	}
}

// Init enables Bloom so the example starts with one easy-to-see effect; the
// control panel can enable the remaining effects independently.
func (g *Game) Init(*runtime.UpdateContext) {
	for i := range g.State.Enabled {
		g.State.Enabled[i] = false
	}
	g.State.Enabled[effectBloom] = true
	g.State.Selected = effectBloom
	g.State.ShowParameters = true
	g.State.VignetteColor = render.RGBA(0.02, 0.03, 0.08, 1)
	g.State.VignetteStrength = 0.8
	g.State.VignetteSoftness = 0.65
	g.State.ColorGradeBrightness = 0.04
	g.State.ColorGradeContrast = 0.16
	g.State.ColorGradeSaturation = 0.25
	g.State.ColorGradeTint = render.RGBA(0.94, 0.98, 1, 1)
	g.State.PixelateSize = 8
	g.State.ChromaticStrength = 2.5
	g.State.BloomThreshold = 0.55
	g.State.BloomIntensity = 2.5
	g.State.BloomPasses = 2
	g.State.ShockwaveCenter = math2d.Vec2{X: 810, Y: 365}
	g.State.ShockwaveRadius = 250
	g.State.ShockwaveWidth = 34
	g.State.ShockwaveStrength = 18
	g.State.LensRainDensity = 0.22
	g.State.LensRainSize = 44
	g.State.LensRainSpeed = 0.15
	g.State.LensRainRefraction = 0.75
	g.State.LensRainSeed = 0x504f5354
	g.State.LensRainSeedText = fmt.Sprintf("0x%X", g.State.LensRainSeed)
}

// Update applies the control panel's caller-owned values to the game state.
func (g *Game) Update(ctx *runtime.UpdateContext, _ float64) {
	ctx.HoldSimulation()
	layout := postfxLayout()
	in := ctx.UI.In(ui.SpaceRef)

	for i := range effectLabels {
		id := effectID(i)
		if in.Clicked(effectSelectID(id), layout.selectors[i]) {
			g.State.Selected = id
		}
		next, changed := in.CheckboxInput(effectToggleID(id), layout.toggles[i], g.State.Enabled[i])
		if changed {
			g.State.Enabled[i] = next
		}
	}
	if in.Clicked("postfx_all", layout.allButton) {
		for i := range g.State.Enabled {
			g.State.Enabled[i] = true
		}
	}
	if in.Clicked("postfx_none", layout.noneButton) {
		for i := range g.State.Enabled {
			g.State.Enabled[i] = false
		}
	}
	if g.State.ShowParameters {
		if in.Clicked("postfx_hide_parameters", layout.parameterToggle) {
			g.State.ShowParameters = false
		}
	} else if in.Clicked("postfx_show_parameters", layout.showParameters) {
		g.State.ShowParameters = true
	}
	if g.State.ShowParameters {
		g.updateSelected(in, layout)
	}
}

// Render draws the high-contrast scene, the configurable post chain, and the
// controls that are intentionally kept above the processed image.
func (g *Game) Render(ctx *runtime.RenderContext) []render.Command {
	commands := drawScene(ctx.Time)
	commands = append(commands, render.PostChain{
		BaseCommand: render.BaseCommand{LayerValue: 1},
		BelowLayer:  ui.LayerBase,
		Effects:     g.effects(ctx.Time),
		Time:        ctx.Time,
	})

	draw := ctx.UI.In(ui.SpaceRef)
	layout := postfxLayout()
	draw.TitledPanel(layout.controls, "POST FX LAB")
	draw.Label(fmt.Sprintf("%d / %d ACTIVE", activeEffects(g.State.Enabled), effectCount), math2d.Vec2{X: layout.controls.Center().X, Y: layout.controls.Min.Y + 72})
	for i, label := range effectMenuLabels {
		id := effectID(i)
		draw.Checkbox(effectToggleID(id), layout.toggles[i], g.State.Enabled[i], label)
		selectLabel := "EDIT"
		if g.State.Selected == id {
			selectLabel = "OPEN"
		}
		draw.Button(effectSelectID(id), selectLabel, layout.selectors[i])
	}
	draw.Button("postfx_all", "ALL", layout.allButton)
	draw.Button("postfx_none", "NONE", layout.noneButton)
	if g.State.ShowParameters {
		draw.DrawText("EDIT opens the complete config", math2d.Vec2{X: layout.controls.Min.X + 24, Y: layout.controls.Max.Y - 30}, render.RGBA8(170, 185, 205, 255), 13, render.AlignLeft, render.BaselineMiddle)
		draw.TitledPanel(layout.parameters, effectTitle(g.State.Selected))
		draw.Button("postfx_hide_parameters", "HIDE", layout.parameterToggle)
		draw.DrawText(effectDescription(g.State.Selected), math2d.Vec2{X: layout.parameters.Min.X + 20, Y: layout.parameters.Min.Y + 72}, render.RGBA8(170, 185, 205, 255), 12, render.AlignLeft, render.BaselineMiddle)
		g.drawSelectedParameters(draw, layout)
	} else {
		draw.Button("postfx_show_parameters", "SHOW PARAMS", layout.showParameters)
		draw.DrawText("parameter panel hidden", math2d.Vec2{X: layout.controls.Min.X + 24, Y: layout.controls.Max.Y - 30}, render.RGBA8(170, 185, 205, 255), 13, render.AlignLeft, render.BaselineMiddle)
	}

	return commands
}

func (g *Game) effects(_ float64) []render.PostEffect {
	effects := make([]render.PostEffect, 0, effectCount)
	if g.State.Enabled[effectShockwave] {
		effects = append(effects, render.Shockwave{
			Center:   g.State.ShockwaveCenter,
			Radius:   g.State.ShockwaveRadius,
			Width:    g.State.ShockwaveWidth,
			Strength: g.State.ShockwaveStrength,
		})
	}
	if g.State.Enabled[effectLensRain] {
		effects = append(effects, render.LensRain{
			Density:    g.State.LensRainDensity,
			Size:       g.State.LensRainSize,
			Speed:      g.State.LensRainSpeed,
			Refraction: g.State.LensRainRefraction,
			Seed:       g.State.LensRainSeed,
		})
	}
	if g.State.Enabled[effectChromatic] {
		effects = append(effects, render.ChromaticAberration{Strength: g.State.ChromaticStrength})
	}
	if g.State.Enabled[effectPixelate] {
		effects = append(effects, render.Pixelate{Size: g.State.PixelateSize})
	}
	if g.State.Enabled[effectBloom] {
		effects = append(effects, render.Bloom{
			Passes:    g.State.BloomPasses,
			Intensity: g.State.BloomIntensity,
			Threshold: g.State.BloomThreshold,
		})
	}
	if g.State.Enabled[effectColorGrade] {
		effects = append(effects, render.ColorGrade{
			Brightness: g.State.ColorGradeBrightness,
			Contrast:   g.State.ColorGradeContrast,
			Saturation: g.State.ColorGradeSaturation,
			Tint:       g.State.ColorGradeTint,
		})
	}
	if g.State.Enabled[effectVignette] {
		effects = append(effects, render.Vignette{
			Color:    g.State.VignetteColor,
			Strength: g.State.VignetteStrength,
			Softness: g.State.VignetteSoftness,
		})
	}
	return effects
}

func (g *Game) updateSelected(in ui.InteractScope, layout postfxHUDLayout) {
	slider := func(id string, rect math2d.Rect, value, min, max, step float64) float64 {
		next, _ := in.SliderInput(id, rect, value, ui.SliderOptions{Min: min, Max: max, Step: step})
		return next
	}
	colorSlider := func(id string, rect math2d.Rect, value *float64) {
		*value = slider(id, rect, *value, 0, 1, 0.01)
	}

	switch g.State.Selected {
	case effectVignette:
		g.State.VignetteStrength = slider("postfx_vignette_strength", layout.parametersSlots[0], g.State.VignetteStrength, 0, 1, 0.01)
		g.State.VignetteSoftness = slider("postfx_vignette_softness", layout.parametersSlots[1], g.State.VignetteSoftness, 0, 1, 0.01)
		colorSlider("postfx_vignette_color_r", layout.parametersSlots[2], &g.State.VignetteColor.R)
		colorSlider("postfx_vignette_color_g", layout.parametersSlots[3], &g.State.VignetteColor.G)
		colorSlider("postfx_vignette_color_b", layout.parametersSlots[4], &g.State.VignetteColor.B)
		colorSlider("postfx_vignette_color_a", layout.parametersSlots[5], &g.State.VignetteColor.A)
	case effectColorGrade:
		g.State.ColorGradeBrightness = slider("postfx_colorgrade_brightness", layout.parametersSlots[0], g.State.ColorGradeBrightness, -1, 1, 0.01)
		g.State.ColorGradeContrast = slider("postfx_colorgrade_contrast", layout.parametersSlots[1], g.State.ColorGradeContrast, -1, 1, 0.01)
		g.State.ColorGradeSaturation = slider("postfx_colorgrade_saturation", layout.parametersSlots[2], g.State.ColorGradeSaturation, -1, 1, 0.01)
		colorSlider("postfx_colorgrade_tint_r", layout.parametersSlots[3], &g.State.ColorGradeTint.R)
		colorSlider("postfx_colorgrade_tint_g", layout.parametersSlots[4], &g.State.ColorGradeTint.G)
		colorSlider("postfx_colorgrade_tint_b", layout.parametersSlots[5], &g.State.ColorGradeTint.B)
		colorSlider("postfx_colorgrade_tint_a", layout.parametersSlots[6], &g.State.ColorGradeTint.A)
	case effectPixelate:
		g.State.PixelateSize = slider("postfx_pixelate_size", layout.parametersSlots[0], g.State.PixelateSize, 0, 64, 1)
	case effectChromatic:
		g.State.ChromaticStrength = slider("postfx_chromatic_strength", layout.parametersSlots[0], g.State.ChromaticStrength, 0, 32, 0.25)
	case effectBloom:
		g.State.BloomThreshold = slider("postfx_bloom_threshold", layout.parametersSlots[0], g.State.BloomThreshold, 0, 1, 0.01)
		g.State.BloomIntensity = slider("postfx_bloom_intensity", layout.parametersSlots[1], g.State.BloomIntensity, 0, 10, 0.05)
		passes := slider("postfx_bloom_passes", layout.parametersSlots[2], float64(g.State.BloomPasses), 0, 5, 1)
		g.State.BloomPasses = int(math.Round(passes))
	case effectShockwave:
		g.State.ShockwaveCenter.X = slider("postfx_shockwave_center_x", layout.parametersSlots[0], g.State.ShockwaveCenter.X, 0, viewW, 1)
		g.State.ShockwaveCenter.Y = slider("postfx_shockwave_center_y", layout.parametersSlots[1], g.State.ShockwaveCenter.Y, 0, viewH, 1)
		g.State.ShockwaveRadius = slider("postfx_shockwave_radius", layout.parametersSlots[2], g.State.ShockwaveRadius, 0, 800, 1)
		g.State.ShockwaveWidth = slider("postfx_shockwave_width", layout.parametersSlots[3], g.State.ShockwaveWidth, 0, 200, 1)
		g.State.ShockwaveStrength = slider("postfx_shockwave_strength", layout.parametersSlots[4], g.State.ShockwaveStrength, 0, 100, 1)
	case effectLensRain:
		g.State.LensRainDensity = slider("postfx_lensrain_density", layout.parametersSlots[0], g.State.LensRainDensity, 0, 1, 0.01)
		g.State.LensRainSize = slider("postfx_lensrain_size", layout.parametersSlots[1], g.State.LensRainSize, 0, 160, 1)
		g.State.LensRainSpeed = slider("postfx_lensrain_speed", layout.parametersSlots[2], g.State.LensRainSpeed, 0, 1, 0.01)
		g.State.LensRainRefraction = slider("postfx_lensrain_refraction", layout.parametersSlots[3], g.State.LensRainRefraction, 0, 2, 0.01)
		seed := in.TextInput("postfx_lensrain_seed", layout.parametersSlots[4], g.State.LensRainSeedText, ui.TextInputOptions{Placeholder: "0x504F5354", MaxRunes: 18})
		if seed.Changed {
			g.State.LensRainSeedText = seed.Text
			if parsed, err := strconv.ParseUint(strings.TrimSpace(seed.Text), 0, 64); err == nil {
				g.State.LensRainSeed = parsed
			}
		}
	}
}

func (g *Game) drawSelectedParameters(draw ui.DrawScope, layout postfxHUDLayout) {
	drawSlider := func(id, label string, rect math2d.Rect, value float64, min, max, step float64, format string) {
		draw.DrawText(fmt.Sprintf(label+"  "+format, value), math2d.Vec2{X: rect.Min.X, Y: rect.Min.Y - 10}, render.RGBA8(215, 225, 240, 255), 14, render.AlignLeft, render.BaselineMiddle)
		draw.Slider(id, rect, value, ui.SliderOptions{Min: min, Max: max, Step: step})
	}
	drawColor := func(prefix, label string, rect math2d.Rect, value float64) {
		drawSlider(prefix, label, rect, value, 0, 1, 0.01, "%.2f")
	}

	switch g.State.Selected {
	case effectVignette:
		drawSlider("postfx_vignette_strength", "strength", layout.parametersSlots[0], g.State.VignetteStrength, 0, 1, 0.01, "%.2f")
		drawSlider("postfx_vignette_softness", "softness", layout.parametersSlots[1], g.State.VignetteSoftness, 0, 1, 0.01, "%.2f")
		drawColor("postfx_vignette_color_r", "color R", layout.parametersSlots[2], g.State.VignetteColor.R)
		drawColor("postfx_vignette_color_g", "color G", layout.parametersSlots[3], g.State.VignetteColor.G)
		drawColor("postfx_vignette_color_b", "color B", layout.parametersSlots[4], g.State.VignetteColor.B)
		drawColor("postfx_vignette_color_a", "color A", layout.parametersSlots[5], g.State.VignetteColor.A)
	case effectColorGrade:
		drawSlider("postfx_colorgrade_brightness", "brightness", layout.parametersSlots[0], g.State.ColorGradeBrightness, -1, 1, 0.01, "%.2f")
		drawSlider("postfx_colorgrade_contrast", "contrast", layout.parametersSlots[1], g.State.ColorGradeContrast, -1, 1, 0.01, "%.2f")
		drawSlider("postfx_colorgrade_saturation", "saturation", layout.parametersSlots[2], g.State.ColorGradeSaturation, -1, 1, 0.01, "%.2f")
		drawColor("postfx_colorgrade_tint_r", "tint R", layout.parametersSlots[3], g.State.ColorGradeTint.R)
		drawColor("postfx_colorgrade_tint_g", "tint G", layout.parametersSlots[4], g.State.ColorGradeTint.G)
		drawColor("postfx_colorgrade_tint_b", "tint B", layout.parametersSlots[5], g.State.ColorGradeTint.B)
		drawColor("postfx_colorgrade_tint_a", "tint A", layout.parametersSlots[6], g.State.ColorGradeTint.A)
	case effectPixelate:
		drawSlider("postfx_pixelate_size", "size (pixels)", layout.parametersSlots[0], g.State.PixelateSize, 0, 64, 1, "%.0f")
	case effectChromatic:
		drawSlider("postfx_chromatic_strength", "strength (pixels)", layout.parametersSlots[0], g.State.ChromaticStrength, 0, 32, 0.25, "%.2f")
	case effectBloom:
		drawSlider("postfx_bloom_threshold", "threshold", layout.parametersSlots[0], g.State.BloomThreshold, 0, 1, 0.01, "%.2f")
		drawSlider("postfx_bloom_intensity", "intensity", layout.parametersSlots[1], g.State.BloomIntensity, 0, 10, 0.05, "%.2f")
		drawSlider("postfx_bloom_passes", "passes (0 = default)", layout.parametersSlots[2], float64(g.State.BloomPasses), 0, 5, 1, "%.0f")
	case effectShockwave:
		drawSlider("postfx_shockwave_center_x", "center X", layout.parametersSlots[0], g.State.ShockwaveCenter.X, 0, viewW, 1, "%.0f")
		drawSlider("postfx_shockwave_center_y", "center Y", layout.parametersSlots[1], g.State.ShockwaveCenter.Y, 0, viewH, 1, "%.0f")
		drawSlider("postfx_shockwave_radius", "radius", layout.parametersSlots[2], g.State.ShockwaveRadius, 0, 800, 1, "%.0f")
		drawSlider("postfx_shockwave_width", "width", layout.parametersSlots[3], g.State.ShockwaveWidth, 0, 200, 1, "%.0f")
		drawSlider("postfx_shockwave_strength", "strength", layout.parametersSlots[4], g.State.ShockwaveStrength, 0, 100, 1, "%.0f")
	case effectLensRain:
		drawSlider("postfx_lensrain_density", "density", layout.parametersSlots[0], g.State.LensRainDensity, 0, 1, 0.01, "%.2f")
		drawSlider("postfx_lensrain_size", "size (pixels)", layout.parametersSlots[1], g.State.LensRainSize, 0, 160, 1, "%.0f")
		drawSlider("postfx_lensrain_speed", "speed", layout.parametersSlots[2], g.State.LensRainSpeed, 0, 1, 0.01, "%.2f")
		drawSlider("postfx_lensrain_refraction", "refraction", layout.parametersSlots[3], g.State.LensRainRefraction, 0, 2, 0.01, "%.2f")
		draw.DrawText("seed (decimal or 0x hex)", math2d.Vec2{X: layout.parametersSlots[4].Min.X, Y: layout.parametersSlots[4].Min.Y - 10}, render.RGBA8(215, 225, 240, 255), 14, render.AlignLeft, render.BaselineMiddle)
		draw.TextInput("postfx_lensrain_seed", layout.parametersSlots[4], g.State.LensRainSeedText, ui.TextInputOptions{Placeholder: "0x504F5354", MaxRunes: 18})
	}
}

func effectDescription(id effectID) string {
	switch id {
	case effectVignette:
		return "edge color / strength / softness"
	case effectColorGrade:
		return "brightness / contrast / saturation / tint"
	case effectPixelate:
		return "square sample size"
	case effectChromatic:
		return "radial RGB offset"
	case effectBloom:
		return "threshold / intensity / passes"
	case effectShockwave:
		return "center / radius / width / strength"
	case effectLensRain:
		return "density / size / speed / refraction / seed"
	default:
		return ""
	}
}

func effectTitle(id effectID) string {
	switch id {
	case effectVignette:
		return "VIGNETTE"
	case effectColorGrade:
		return "COLOR GRADE"
	case effectPixelate:
		return "PIXELATE"
	case effectChromatic:
		return "CHROMATIC"
	case effectBloom:
		return "BLOOM"
	case effectShockwave:
		return "SHOCKWAVE"
	case effectLensRain:
		return "LENS RAIN"
	default:
		return "POST FX"
	}
}

func drawScene(time float64) []render.Command {
	commands := []render.Command{
		render.DrawRect{
			BaseCommand: render.BaseCommand{LayerValue: -20},
			Rect:        math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: viewW, Y: viewH}),
			Fill:        render.RGBA8(8, 12, 22, 255),
		},
	}
	for row := range 8 {
		for col := range 10 {
			fill := render.RGBA(
				0.025+float64((row+col)%3)*0.012,
				0.05+float64(col%4)*0.012,
				0.11+float64(row%3)*0.018,
				1,
			)
			commands = append(commands, render.DrawRect{
				BaseCommand: render.BaseCommand{LayerValue: -19},
				Rect: math2d.NewRect(
					math2d.Vec2{X: 310 + float64(col)*68, Y: 34 + float64(row)*82},
					math2d.Vec2{X: 60, Y: 74},
				),
				Fill: fill,
			})
		}
	}
	for row := range 7 {
		commands = append(commands, render.DrawLine{
			BaseCommand: render.BaseCommand{LayerValue: -18},
			From:        math2d.Vec2{X: 320, Y: 90 + float64(row)*82},
			To:          math2d.Vec2{X: 1000, Y: 90 + float64(row)*82},
			Width:       1,
			Color:       render.RGBA(0.12, 0.24, 0.45, 0.55),
		})
	}
	for col := range 9 {
		commands = append(commands, render.DrawLine{
			BaseCommand: render.BaseCommand{LayerValue: -18},
			From:        math2d.Vec2{X: 330 + float64(col)*78, Y: 70},
			To:          math2d.Vec2{X: 330 + float64(col)*78, Y: 625},
			Width:       1,
			Color:       render.RGBA(0.12, 0.24, 0.45, 0.55),
		})
	}

	commands = append(commands,
		render.DrawRect{
			BaseCommand: render.BaseCommand{LayerValue: -10},
			Rect:        math2d.NewRect(math2d.Vec2{X: 300, Y: 48}, math2d.Vec2{X: 700, Y: 620}),
			Fill:        render.RGBA(0.025, 0.04, 0.09, 0.86),
			Stroke:      render.RGBA(0.2, 0.38, 0.7, 1),
			StrokeWidth: 3,
		},
		render.DrawCircle{
			BaseCommand: render.BaseCommand{LayerValue: -5},
			Center:      math2d.Vec2{X: 520, Y: 275},
			Radius:      132,
			Fill:        render.RGBA(0.02, 0.32, 0.9, 0.85),
			Stroke:      render.RGBA8(190, 235, 255, 255),
			StrokeWidth: 5,
		},
		render.DrawCircle{
			BaseCommand: render.BaseCommand{LayerValue: -4},
			Center:      math2d.Vec2{X: 820, Y: 260},
			Radius:      92,
			Fill:        render.RGBA(1, 0.22, 0.04, 0.9),
			Stroke:      render.RGBA8(255, 230, 120, 255),
			StrokeWidth: 5,
		},
		render.DrawCircle{
			BaseCommand: render.BaseCommand{LayerValue: -3},
			Center:      math2d.Vec2{X: 760, Y: 515},
			Radius:      70,
			Fill:        render.RGBA(0.05, 0.92, 0.58, 0.8),
			Stroke:      render.RGBA8(200, 255, 224, 255),
			StrokeWidth: 4,
		},
		render.DrawLine{
			BaseCommand: render.BaseCommand{LayerValue: -2},
			From:        math2d.Vec2{X: 330, Y: 590},
			To:          math2d.Vec2{X: 960, Y: 82},
			Width:       8,
			Color:       render.RGBA(0.35, 1, 0.8, 0.95),
		},
		render.DrawRect{
			BaseCommand: render.BaseCommand{LayerValue: -1},
			Rect:        math2d.NewRect(math2d.Vec2{X: 640, Y: 120}, math2d.Vec2{X: 170, Y: 42}),
			Fill:        render.RGBA8(255, 239, 90, 255),
		},
		render.DrawArc{
			BaseCommand: render.BaseCommand{LayerValue: -6},
			Center:      math2d.Vec2{X: 520, Y: 275},
			Radius:      158,
			StartAngle:  0.25,
			EndAngle:    2.45,
			Width:       8,
			Color:       render.RGBA8(255, 120, 80, 230),
		},
		render.DrawArc{
			BaseCommand: render.BaseCommand{LayerValue: -6},
			Center:      math2d.Vec2{X: 520, Y: 275},
			Radius:      174,
			StartAngle:  3.3,
			EndAngle:    5.35,
			Width:       5,
			Color:       render.RGBA8(80, 180, 255, 220),
		},
		render.DrawText{
			BaseCommand: render.BaseCommand{LayerValue: -7},
			Position:    math2d.Vec2{X: 325, Y: 76},
			Text:        "SIGNAL FIELD // 07",
			Color:       render.RGBA8(175, 210, 255, 255),
			Size:        17,
			Align:       render.AlignLeft,
			Baseline:    render.BaselineMiddle,
		},
		render.DrawText{
			BaseCommand: render.BaseCommand{LayerValue: -7},
			Position:    math2d.Vec2{X: 970, Y: 612},
			Text:        "LIVE / 60 FPS",
			Color:       render.RGBA8(105, 245, 210, 255),
			Size:        14,
			Align:       render.AlignRight,
			Baseline:    render.BaselineMiddle,
		},
	)
	for i := range 5 {
		barHeight := 28 + float64((i*17)%82)
		commands = append(commands, render.DrawRect{
			BaseCommand: render.BaseCommand{LayerValue: -2},
			Rect:        math2d.NewRect(math2d.Vec2{X: 760 + float64(i)*34, Y: 570 - barHeight}, math2d.Vec2{X: 22, Y: barHeight}),
			Fill:        render.RGBA(0.12+float64(i)*0.12, 0.35+float64(i)*0.08, 0.95-float64(i)*0.1, 0.9),
		})
	}

	for i := range 9 {
		phase := math.Mod(time*0.6+float64(i)*0.17, 1)
		commands = append(commands, render.DrawCircle{
			BaseCommand: render.BaseCommand{LayerValue: 0},
			Center: math2d.Vec2{
				X: 350 + float64(i)*70,
				Y: 100 + phase*470,
			},
			Radius: 7 + float64(i%3)*3,
			Fill:   render.RGBA8(255, 255, 255, 255),
		})
	}
	return commands
}

type postfxHUDLayout struct {
	controls, parameters, allButton, noneButton, parameterToggle, showParameters math2d.Rect
	selectors, toggles                                                           [effectCount]math2d.Rect
	parametersSlots                                                              [7]math2d.Rect
}

func postfxLayout() postfxHUDLayout {
	controls := math2d.NewRect(math2d.Vec2{X: 24, Y: 24}, math2d.Vec2{X: 260, Y: 672})
	parameters := math2d.NewRect(math2d.Vec2{X: 1028, Y: 24}, math2d.Vec2{X: 228, Y: 566})
	layout := postfxHUDLayout{
		controls:        controls,
		parameters:      parameters,
		allButton:       math2d.NewRect(math2d.Vec2{X: 40, Y: 380}, math2d.Vec2{X: 105, Y: 30}),
		noneButton:      math2d.NewRect(math2d.Vec2{X: 155, Y: 380}, math2d.Vec2{X: 105, Y: 30}),
		parameterToggle: math2d.NewRect(math2d.Vec2{X: 1182, Y: 31}, math2d.Vec2{X: 60, Y: 22}),
		showParameters:  math2d.NewRect(math2d.Vec2{X: 40, Y: 430}, math2d.Vec2{X: 220, Y: 30}),
	}
	for i := range layout.selectors {
		y := 100 + float64(i)*38
		layout.toggles[i] = math2d.NewRect(
			math2d.Vec2{X: 40, Y: y},
			math2d.Vec2{X: 162, Y: 30},
		)
		layout.selectors[i] = math2d.NewRect(
			math2d.Vec2{X: 206, Y: y},
			math2d.Vec2{X: 54, Y: 30},
		)
	}
	for i := range layout.parametersSlots {
		layout.parametersSlots[i] = math2d.NewRect(
			math2d.Vec2{X: 1044, Y: 130 + float64(i)*64},
			math2d.Vec2{X: 196, Y: 24},
		)
	}
	return layout
}

func effectToggleID(id effectID) string {
	return fmt.Sprintf("postfx_toggle_%d", id)
}

func effectSelectID(id effectID) string {
	return fmt.Sprintf("postfx_select_%d", id)
}

func activeEffects(enabled [effectCount]bool) int {
	count := 0
	for _, on := range enabled {
		if on {
			count++
		}
	}
	return count
}
