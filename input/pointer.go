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

// PointerEvent preserves the event position for edge-crossing clicks.
type PointerEvent struct {
	Kind   EventKind
	Button PointerButton
	Pos    math2d.Vec2
}

// PointerFrame is the coalesced pointer input for one tick.
// Moves collapse to the final position; button transitions keep their origin.
type PointerFrame struct {
	Pos     math2d.Vec2
	Inside  bool
	Buttons [3]bool
	Wheel   math2d.Vec2
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
