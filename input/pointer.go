package input

import (
	"github.com/dhannyell/frameforge/math2d"
)

type PointerButton uint8

const (
	PointerPrimary PointerButton = iota
	PointerSecondary
	PointerMiddle
	PointerUnknown
)

// PointerEvent is a pointer event with a position. It carries the position instead of
// just a boolean — necessary for down-outside/up-inside and down-inside/up-outside.
type PointerEvent struct {
	Kind   EventKind
	Button PointerButton
	Pos    math2d.Vec2
}

// PointerFrame is the coalesced set of events from a single tick, consumed in Update.
// Moves collapse to the final position; button transitions (rare) are preserved
// along with their position. inside controls hover/leave.
type PointerFrame struct {
	Pos     math2d.Vec2
	Inside  bool
	Buttons [3]bool
	Events  []PointerEvent
}

type PointerState struct {
	pos    math2d.Vec2
	button [3]bool
	inside bool
}

func (p *PointerState) Pos() math2d.Vec2 {
	return p.pos
}

func (p *PointerState) SetPos(pos math2d.Vec2) {
	p.pos = pos
}

func (p *PointerState) SetButton(button PointerButton, value bool) {
	if button >= PointerButton(len(p.button)) {
		return
	}
	p.button[button] = value
}

func (p *PointerState) Button(button PointerButton) bool {
	if button >= PointerButton(len(p.button)) {
		return false
	}
	return p.button[button]
}

func (p *PointerState) Buttons() [3]bool {
	return p.button
}

func (p *PointerState) Inside() bool {
	return p.inside
}

func (p *PointerState) SetInside(inside bool) {
	p.inside = inside
}

func (p *PointerState) Clone() PointerState {
	return PointerState{
		pos:    p.Pos(),
		button: p.button,
		inside: p.inside,
	}
}
