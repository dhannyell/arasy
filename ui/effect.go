package ui

import (
	"math"

	"github.com/dhannyell/frameforge/ease"
	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
)

const defaultEffectDuration = 1.0

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

type ToastOptions struct {
	Key      uint64
	Duration float64

	Layer int

	Fill        render.Color
	Border      render.Color
	StrokeWidth float64
	TextColor   render.Color
	TextSize    float64

	FromY    float64
	UseFromY bool

	EnterStart float64
	EnterEnd   float64
	EnterEase  ease.Kind
	ExitStart  float64
	ExitEnd    float64
	ExitEase   ease.Kind

	PulseStart     float64
	PulseEnd       float64
	PulseAmount    float64
	PulseFrequency float64
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
	opts = fx.toastDefaults(rect, opts)
	effect := fx.ui.Effect(id, EffectOptions{Key: opts.Key, Duration: opts.Duration})
	if effect.Done() {
		return false
	}

	drawRect := effect.SlideY(rect, opts.FromY, rect.Min.Y, opts.EnterStart, opts.EnterEnd, opts.EnterEase)
	alpha := effect.FadeOut(opts.ExitStart, opts.ExitEnd, opts.ExitEase)
	scale := effect.Pulse(opts.PulseStart, opts.PulseEnd, opts.PulseAmount, opts.PulseFrequency)

	layer := opts.Layer
	if layer == 0 {
		layer = fx.ui.nextLayer()
	}
	fx.ui.commands = append(fx.ui.commands,
		render.DrawRect{
			BaseCommand: render.BaseCommand{LayerValue: layer},
			Rect:        fx.ui.rectToWorld(drawRect, space),
			Fill:        multiplyAlpha(opts.Fill, alpha),
			Stroke:      multiplyAlpha(opts.Border, alpha),
			StrokeWidth: fx.ui.sizeToWorld(opts.StrokeWidth, space),
		},
		render.DrawText{
			BaseCommand: render.BaseCommand{LayerValue: layer + 1},
			Position:    fx.ui.toWorld(drawRect.Center(), space),
			Text:        text,
			Color:       multiplyAlpha(opts.TextColor, alpha),
			Size:        fx.ui.sizeToWorld(opts.TextSize*scale, space),
			Align:       render.AlignCenter,
			Baseline:    render.BaselineMiddle,
		},
	)
	return true
}

func (fx FX) toastDefaults(rect math2d.Rect, opts ToastOptions) ToastOptions {
	if opts.Duration <= 0 {
		opts.Duration = 2.0
	}
	if opts.EnterEnd <= 0 {
		opts.EnterEnd = 0.25
	}
	if opts.ExitEnd <= 0 {
		opts.ExitEnd = opts.Duration
	}
	if opts.ExitStart <= 0 {
		opts.ExitStart = opts.Duration - 0.25
		if opts.ExitStart < opts.EnterEnd {
			opts.ExitStart = opts.EnterEnd
		}
	}
	if !opts.UseFromY {
		opts.FromY = rect.Min.Y - rect.Height()*2
	}
	if opts.Fill == (render.Color{}) {
		opts.Fill = fx.ui.theme.Panel.Background
	}
	if opts.Border == (render.Color{}) {
		opts.Border = fx.ui.theme.Panel.Border
	}
	if opts.StrokeWidth == 0 {
		opts.StrokeWidth = fx.ui.theme.Panel.BorderWidth
	}
	if opts.TextColor == (render.Color{}) {
		opts.TextColor = fx.ui.theme.Label.Color
	}
	if opts.TextSize == 0 {
		opts.TextSize = fx.ui.theme.Label.Size
	}
	return opts
}

func multiplyAlpha(c render.Color, alpha float64) render.Color {
	c.A *= alpha
	return c
}
