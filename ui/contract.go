package ui

import (
	"github.com/dhannyell/arasy/ease"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
)

// Interactor is the interface for Update: deterministic queries that read the
// tick's input.PointerFrame and can mutate ui.State. Implemented by *UI
// (constructed via ui.NewUpdate).
type Interactor interface {
	In(space Space) InteractScope

	RowHeight() float64
	HeaderHeight() float64
	TableBodyRect(rect math2d.Rect) math2d.Rect
}

// Drawer emits UI render commands and exposes animation primitives.
type Drawer interface {
	In(space Space) DrawScope

	RowHeight() float64
	HeaderHeight() float64
	TableBodyRect(rect math2d.Rect) math2d.Rect
	SetFont(id render.FontID)

	// Animation primitives — state-driven

	Approach(id string, target, speed float64) float64
	Spring(id string, target float64, cfg SpringConfig) float64
	SpringScale(id string, target float64, cfg SpringConfig, rect math2d.Rect) math2d.Rect

	// Animation primitives — time-driven

	Tween(id string, dur float64, k ease.Kind) float64
	Clock(id string) float64
	Reset(id string)

	// Sequence — multistep scripted animation

	SequenceKeyed(id string, key uint64) Sequence

	// Mid/high-level animation helpers

	Effect(id string, opts EffectOptions) Effect

	// Command groups — render-only container transforms

	Emit(cmds ...render.Command)
	BeginGroup(opts GroupOptions)
	EndGroup()

	// Screen transitions — keyed screen-change animation

	ScreenTransition(id string, key uint64, opts TransitionOptions) ScreenTransition
}

var (
	_ Drawer     = drawFace{}
	_ Interactor = interactFace{}
)
