package ui

import (
	"testing"

	"github.com/dhannyell/arasy/input"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
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

	if !u.clickedIn("btn", rect, SpaceRef) {
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

	if u.clickedIn("btn", rect, SpaceRef) {
		t.Fatal("did not expect click when up is outside the rect")
	}
	if state.activeID != noID {
		t.Fatalf("activeID = %v, want noID after up-outside releases", state.activeID)
	}
}

func TestClickedFalseWithNoEvents(t *testing.T) {
	cam := testInteractCamera()
	u := NewUpdate(&State{}, pointerFrame(), cam, DefaultTheme(), cam.Viewport)
	if u.clickedIn("btn", btnRect(), SpaceRef) {
		t.Fatal("did not expect click with no pointer events")
	}
}

func TestClickedIgnoresUnhandledEventKind(t *testing.T) {
	state := &State{}
	rect := btnRect()
	cam := testInteractCamera()
	u := NewUpdate(state, pointerFrame(
		input.PointerEvent{Kind: input.EventButtonDown, Button: input.PointerPrimary, Pos: math2d.Vec2{X: 150, Y: 120}},
		input.PointerEvent{Kind: input.EventPointerMove, Button: input.PointerPrimary, Pos: math2d.Vec2{X: 150, Y: 120}},
		input.PointerEvent{Kind: input.EventButtonUp, Button: input.PointerPrimary, Pos: math2d.Vec2{X: 150, Y: 120}},
	), cam, DefaultTheme(), cam.Viewport)

	if !u.clickedIn("btn", rect, SpaceRef) {
		t.Fatal("expected click to still complete when an unhandled event kind is interleaved")
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

	if u.clickedIn("btn", rect, SpaceRef) {
		t.Fatal("did not expect click from a non-primary button")
	}
	if state.activeID != noID {
		t.Fatalf("activeID = %v, want noID; secondary button must not capture", state.activeID)
	}
}

func TestFocusableTabCyclesFocusAtEndFrame(t *testing.T) {
	state := &State{focusID: HashID("one")}
	cam := testInteractCamera()
	u := NewUpdateWithInput(state, pointerFrame(), inputStateJustPressed(input.ActionFocusNext), cam, DefaultTheme(), cam.Viewport)

	u.focusableIn("one", btnRect(), SpaceRef)
	u.focusableIn("two", btnRect(), SpaceRef)
	u.EndFrame()

	if state.focusID != HashID("two") {
		t.Fatalf("focusID = %v, want second focusable", state.focusID)
	}
}

func TestFocusableTabSelectsFirstWhenNothingFocused(t *testing.T) {
	state := &State{}
	cam := testInteractCamera()
	u := NewUpdateWithInput(state, pointerFrame(), inputStateJustPressed(input.ActionFocusNext), cam, DefaultTheme(), cam.Viewport)

	u.focusableIn("one", btnRect(), SpaceRef)
	u.focusableIn("two", btnRect(), SpaceRef)
	u.EndFrame()

	if state.focusID != HashID("one") {
		t.Fatalf("focusID = %v, want first focusable", state.focusID)
	}
}

func TestActivatedReturnsTrueForFocusedConfirm(t *testing.T) {
	state := &State{focusID: HashID("btn")}
	cam := testInteractCamera()
	u := NewUpdateWithInput(state, pointerFrame(), inputStateJustPressed(input.ActionConfirm), cam, DefaultTheme(), cam.Viewport)

	if !u.activatedIn("btn", btnRect(), SpaceRef) {
		t.Fatal("expected focused widget to activate on confirm")
	}
	if u.activatedIn("other", btnRect(), SpaceRef) {
		t.Fatal("did not expect unfocused widget to activate")
	}
}

func TestCancelClearsFocusAtEndFrame(t *testing.T) {
	state := &State{focusID: HashID("btn")}
	cam := testInteractCamera()
	u := NewUpdateWithInput(state, pointerFrame(), inputStateJustPressed(input.ActionCancel), cam, DefaultTheme(), cam.Viewport)

	u.focusableIn("btn", btnRect(), SpaceRef)
	u.EndFrame()

	if state.focusID != noID {
		t.Fatalf("focusID = %v, want noID", state.focusID)
	}
}

func TestHoveredTrueWhenCursorInsideRect(t *testing.T) {
	cam := testInteractCamera()
	u := New(&State{}, math2d.Vec2{X: 150, Y: 120}, true, cam, DefaultTheme(), cam.Viewport, &AnimStore{}, 0, 0)

	if !u.hoveredIn(btnRect(), SpaceRef) {
		t.Fatal("expected Hovered to be true when cursor is inside the rect")
	}
}

func TestHoveredFalseWhenCursorOutsideRect(t *testing.T) {
	cam := testInteractCamera()
	u := New(&State{}, math2d.Vec2{X: 10, Y: 10}, true, cam, DefaultTheme(), cam.Viewport, &AnimStore{}, 0, 0)

	if u.hoveredIn(btnRect(), SpaceRef) {
		t.Fatal("expected Hovered to be false when cursor is outside the rect")
	}
}

func TestHoveredFalseWhenCursorNotInside(t *testing.T) {
	cam := testInteractCamera()
	u := New(&State{}, math2d.Vec2{X: 150, Y: 120}, false, cam, DefaultTheme(), cam.Viewport, &AnimStore{}, 0, 0)

	if u.hoveredIn(btnRect(), SpaceRef) {
		t.Fatal("expected Hovered to be false when pointer is not inside the viewport")
	}
}

func inputStateJustPressed(action input.Action) input.State {
	mapper := input.NewMapper([]input.Binding{
		{
			Source: input.Keyboard(input.KeyEnter),
			Action: action,
			Scale:  1,
		},
	})
	mapper.Apply(input.Event{
		Source: input.Keyboard(input.KeyEnter),
		Kind:   input.EventButtonDown,
	})
	return mapper.State()
}
