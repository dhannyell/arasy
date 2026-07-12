package ui

import (
	"math"

	"github.com/dhannyell/frameforge/ease"
	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
)

const defaultEffectDuration = 1.0

const (
	defaultToastDuration = 2.0
	toastSlideStart      = 0.0
	toastSlideEnd        = 0.25
	toastExitDuration    = 0.25
	toastPulseStart      = 0.0
	toastPulseEnd        = 0.0
	toastPulseAmount     = 0.0
	toastPulseHz         = 0.0
)

// EffectOptions configures UI.Effect, the mid-level animation API.
// Key follows SequenceKeyed semantics: key=0 means inactive and clears the
// retained trigger, while a new non-zero key restarts the effect.
type EffectOptions struct {
	Key      uint64
	Duration float64
}

// Effect is a small timeline wrapper over SequenceKeyed. It keeps the low-level
// Track API available while naming common visual transforms.
type Effect struct {
	ui       *UI
	seq      Sequence
	duration float64
}

func (u *UI) Effect(id string, opts EffectOptions) Effect {
	duration := opts.Duration
	if duration <= 0 {
		duration = defaultEffectDuration
	}
	return Effect{
		ui:       u,
		seq:      u.SequenceKeyed(id, opts.Key),
		duration: duration,
	}
}

func (e Effect) Done() bool { return e.seq.Done(e.duration) }

func (e Effect) Track(start, end float64, k ease.Kind) Track {
	return e.seq.Track(start, end, k)
}

func (e Effect) FadeIn(start, end float64, k ease.Kind) float64 {
	return e.Track(start, end, k).Value()
}

func (e Effect) FadeOut(start, end float64, k ease.Kind) float64 {
	return 1 - e.Track(start, end, k).Value()
}

func (e Effect) SlideY(rect math2d.Rect, fromY, toY, start, end float64, k ease.Kind) math2d.Rect {
	y := e.Track(start, end, k).Lerp(fromY, toY)
	return math2d.NewRect(math2d.Vec2{X: rect.Min.X, Y: y}, rect.Size())
}

func (e Effect) Pulse(start, end, amount, frequency float64) float64 {
	hold := e.Track(start, end, ease.Linear).Value()
	if e.ui == nil || hold == 0 || amount == 0 || frequency == 0 {
		return 1
	}
	return 1 + amount*math.Sin(e.ui.time*frequency)*hold
}

// FX is the high-level animation facade for ready-made UI effects.
type FX struct {
	ui *UI
}

func (u *UI) FX() FX { return FX{ui: u} }

// ToastStyle overrides the toast appearance. The zero value pulls every
// field from the active Theme (Panel for the box, Label for the text).
type ToastStyle struct {
	Fill        render.Color
	Border      render.Color
	StrokeWidth float64
	TextColor   render.Color
	TextSize    float64
}

// ToastOptions configures FX.Toast. Key follows SequenceKeyed semantics:
// key=0 means inactive; a new non-zero key restarts the toast. The motion
// recipe (slide-in, fade-out, pulse) is fixed by design; for custom motion,
// drop down to UI.Effect and compose the primitives directly.
type ToastOptions struct {
	Key      uint64
	Duration float64 // 0 = default (2s)
	Layer    int     // 0 = auto (next UI layer)
	Style    ToastStyle
}

// Toast renders a ready-made panel+text callout in reference-canvas space.
// It returns false when the keyed effect is inactive or done.
func (fx FX) Toast(id, text string, rect math2d.Rect, opts ToastOptions) bool {
	return fx.ToastIn(id, text, rect, SpaceRef, opts)
}

// ToastIn renders a ready-made panel+text callout in the requested UI space.
// For custom world-space effects, prefer UI.Effect and emit render commands
// from the game so the commands use the game's camera directly.
func (fx FX) ToastIn(id, text string, rect math2d.Rect, space Space, opts ToastOptions) bool {
	if fx.ui == nil {
		return false
	}
	opts = fx.toastDefaults(opts)
	effect := fx.ui.Effect(id, EffectOptions{Key: opts.Key, Duration: opts.Duration})
	if effect.Done() {
		return false
	}

	fromY := rect.Min.Y - rect.Height()*2
	exitStart := opts.Duration - toastExitDuration
	if exitStart < toastSlideEnd {
		exitStart = toastSlideEnd
	}

	drawRect := effect.SlideY(rect, fromY, rect.Min.Y, toastSlideStart, toastSlideEnd, ease.Linear)
	alpha := effect.FadeOut(exitStart, opts.Duration, ease.Linear)
	scale := effect.Pulse(toastPulseStart, toastPulseEnd, toastPulseAmount, toastPulseHz)

	layer := opts.Layer
	if layer == 0 {
		layer = fx.ui.nextLayer()
	}
	fx.ui.commands = append(fx.ui.commands,
		render.DrawRect{
			BaseCommand: render.BaseCommand{LayerValue: layer},
			Rect:        fx.ui.rectToWorld(drawRect, space),
			Fill:        multiplyAlpha(opts.Style.Fill, alpha),
			Stroke:      multiplyAlpha(opts.Style.Border, alpha),
			StrokeWidth: fx.ui.sizeToWorld(opts.Style.StrokeWidth, space),
		},
		render.DrawText{
			BaseCommand: render.BaseCommand{LayerValue: layer + 1},
			Position:    fx.ui.toWorld(drawRect.Center(), space),
			Text:        text,
			Color:       multiplyAlpha(opts.Style.TextColor, alpha),
			Size:        fx.ui.sizeToWorld(opts.Style.TextSize*scale, space),
			Font:        fx.ui.theme.Font,
			Align:       render.AlignCenter,
			Baseline:    render.BaselineMiddle,
		},
	)
	return true
}

func (fx FX) toastDefaults(opts ToastOptions) ToastOptions {
	if opts.Duration <= 0 {
		opts.Duration = defaultToastDuration
	}
	if opts.Style.Fill == (render.Color{}) {
		opts.Style.Fill = fx.ui.theme.Panel.Background
	}
	if opts.Style.Border == (render.Color{}) {
		opts.Style.Border = fx.ui.theme.Panel.Border
	}
	if opts.Style.StrokeWidth == 0 {
		opts.Style.StrokeWidth = fx.ui.theme.Panel.BorderWidth
	}
	if opts.Style.TextColor == (render.Color{}) {
		opts.Style.TextColor = fx.ui.theme.Label.Color
	}
	if opts.Style.TextSize == 0 {
		opts.Style.TextSize = fx.ui.theme.Label.Size
	}
	return opts
}

func multiplyAlpha(c render.Color, alpha float64) render.Color {
	c.A *= alpha
	return c
}
