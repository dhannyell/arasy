package ui

import (
	"github.com/dhannyell/frameforge/ease"
	"github.com/dhannyell/frameforge/math2d"
)

// Interactor is the interface for Update: deterministic queries that read the
// tick's PointerFrame and can mutate State.activeID. Implemented by *UI
// (constructed via NewUpdate).
type Interactor interface {
	Clicked(id string, rect math2d.Rect) bool
	ClickedIn(id string, rect math2d.Rect, space Space) bool
}

// Drawer emits UI render commands and exposes animation primitives.
type Drawer interface {
	Button(id, label string, rect math2d.Rect)
	Panel(rect math2d.Rect)
	Label(text string, pos math2d.Vec2)
	ProgressBar(rect math2d.Rect, value float64)

	ButtonIn(id, label string, rect math2d.Rect, space Space)
	PanelIn(rect math2d.Rect, space Space)
	LabelIn(text string, pos math2d.Vec2, space Space)
	ProgressBarIn(rect math2d.Rect, value float64, space Space)

	// Animation primitives — state-driven
	Approach(id string, target, speed float64) float64
	Spring(id string, target float64, cfg SpringConfig) float64

	// Animation primitives — time-driven
	Tween(id string, dur float64, k ease.Kind) float64
	Clock(id string) float64
	Reset(id string)

	// Sequence — multi-step scripted animation
	SequenceKeyed(id string, key uint64) Sequence

	// Mid/high-level animation helpers
	Effect(id string, opts EffectOptions) Effect
	FX() FX
}
