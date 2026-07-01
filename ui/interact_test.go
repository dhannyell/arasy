package ui

import (
	"testing"

	"github.com/dhannyell/frameforge/input"
	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
)

func btnRect() math2d.Rect {
	return math2d.NewRect(math2d.Vec2{X: 100, Y: 100}, math2d.Vec2{X: 200, Y: 140})
}

func pointerFrame(events ...input.PointerEvent) input.PointerFrame {
	return input.PointerFrame{Inside: true, Events: events}
}

// testInteractCamera has World == Viewport, so refCamera's AspectFit resolves
// to an identity mapping — physical pointer positions pass through unchanged,
// which is what these tests assert against.
func testInteractCamera() render.Camera {
	return render.Camera{Viewport: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 400, Y: 400})}
}

func TestClickedTrueWhenDownAndUpInsideSameTick(t *testing.T) {
	state := &State{}
	rect := btnRect()
	cam := testInteractCamera()
	u := NewUpdate(state, pointerFrame(
		input.PointerEvent{Kind: input.EventButtonDown, Button: input.PointerPrimary, Pos: math2d.Vec2{X: 150, Y: 120}},
		input.PointerEvent{Kind: input.EventButtonUp, Button: input.PointerPrimary, Pos: math2d.Vec2{X: 150, Y: 120}},
	), cam, DefaultTheme(), cam.Viewport)

	if !u.Clicked("btn", rect) {
		t.Fatal("expected click when down+up inside the rect in the same tick")
	}
	if state.activeID != noID {
		t.Fatalf("activeID = %v, want noID after click completes", state.activeID)
	}
}

func TestClickedFalseWhenUpOutside(t *testing.T) {
	state := &State{}
	rect := btnRect()
	cam := testInteractCamera()
	u := NewUpdate(state, pointerFrame(
		input.PointerEvent{Kind: input.EventButtonDown, Button: input.PointerPrimary, Pos: math2d.Vec2{X: 150, Y: 120}},
		input.PointerEvent{Kind: input.EventButtonUp, Button: input.PointerPrimary, Pos: math2d.Vec2{X: 10, Y: 10}},
	), cam, DefaultTheme(), cam.Viewport)

	if u.Clicked("btn", rect) {
		t.Fatal("did not expect click when up is outside the rect")
	}
	if state.activeID != noID {
		t.Fatalf("activeID = %v, want noID after up-outside releases", state.activeID)
	}
}

func TestClickedFalseWithNoEvents(t *testing.T) {
	cam := testInteractCamera()
	u := NewUpdate(&State{}, pointerFrame(), cam, DefaultTheme(), cam.Viewport)
	if u.Clicked("btn", btnRect()) {
		t.Fatal("did not expect click with no pointer events")
	}
}

func TestClickedIgnoresNonPrimaryButton(t *testing.T) {
	state := &State{}
	rect := btnRect()
	cam := testInteractCamera()
	u := NewUpdate(state, pointerFrame(
		input.PointerEvent{Kind: input.EventButtonDown, Button: input.PointerSecondary, Pos: math2d.Vec2{X: 150, Y: 120}},
		input.PointerEvent{Kind: input.EventButtonUp, Button: input.PointerSecondary, Pos: math2d.Vec2{X: 150, Y: 120}},
	), cam, DefaultTheme(), cam.Viewport)

	if u.Clicked("btn", rect) {
		t.Fatal("did not expect click from a non-primary button")
	}
	if state.activeID != noID {
		t.Fatalf("activeID = %v, want noID; secondary button must not capture", state.activeID)
	}
}
