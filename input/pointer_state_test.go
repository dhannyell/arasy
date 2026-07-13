package input_test

import (
	"testing"

	"github.com/dhannyell/frameforge/input"
	"github.com/dhannyell/frameforge/math2d"
)

func TestPointerStateClonePreservesState(t *testing.T) {
	var state input.PointerState
	state.SetPos(math2d.Vec2{X: 15, Y: 20})
	state.SetButton(input.PointerPrimary, true)
	state.SetButton(input.PointerMiddle, true)
	state.SetWheel(math2d.Vec2{X: 2, Y: -1})
	state.SetInside(true)

	clone := state.Clone()

	if clone.Pos() != state.Pos() {
		t.Fatalf("clone position = %+v, want %+v", clone.Pos(), state.Pos())
	}
	if clone.Buttons() != state.Buttons() {
		t.Fatalf("clone buttons = %+v, want %+v", clone.Buttons(), state.Buttons())
	}
	if clone.Wheel() != state.Wheel() {
		t.Fatalf("clone wheel = %+v, want %+v", clone.Wheel(), state.Wheel())
	}
	if clone.Inside() != state.Inside() {
		t.Fatalf("clone inside = %v, want %v", clone.Inside(), state.Inside())
	}
}
