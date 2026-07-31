package ui

import (
	"testing"

	"github.com/dhannyell/arasy/input"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
)

func dropdownRect() math2d.Rect {
	return math2d.NewRect(math2d.Vec2{X: 40, Y: 40}, math2d.Vec2{X: 120, Y: 32})
}

func dropdownLabels() []string {
	return []string{"All", "Goalkeepers", "Defenders", "Midfielders", "Forwards"}
}

// clickAt builds the update-phase UI of a tick holding one complete click.
func clickAt(state *State, pos math2d.Vec2) *UI {
	cam := testInteractCamera()
	return NewUpdate(state, pointerFrame(
		input.PointerEvent{Kind: input.EventButtonDown, Button: input.PointerPrimary, Pos: pos},
		input.PointerEvent{Kind: input.EventButtonUp, Button: input.PointerPrimary, Pos: pos},
	), cam, DefaultTheme(), cam.Viewport)
}

func pressAt(state *State, pos math2d.Vec2) *UI {
	cam := testInteractCamera()
	return NewUpdate(state, pointerFrame(
		input.PointerEvent{Kind: input.EventButtonDown, Button: input.PointerPrimary, Pos: pos},
	), cam, DefaultTheme(), cam.Viewport)
}

func openDropdown(t *testing.T, state *State) {
	t.Helper()
	u := clickAt(state, dropdownRect().Center())
	u.dropdownInputIn("filter", dropdownRect(), len(dropdownLabels()), 0, SpaceRef)
	if !state.open[HashID("filter")] {
		t.Fatal("clicking the button did not open the menu")
	}
}

func TestDropdownClickTogglesOpen(t *testing.T) {
	state := &State{}
	openDropdown(t, state)

	u := clickAt(state, dropdownRect().Center())
	u.dropdownInputIn("filter", dropdownRect(), len(dropdownLabels()), 0, SpaceRef)

	if state.open[HashID("filter")] {
		t.Fatal("clicking the button again did not close the menu")
	}
	if state.popupOwner != noID {
		t.Fatal("closing the menu left the popup claim behind")
	}
}

func TestDropdownSelectsItemAndCloses(t *testing.T) {
	state := &State{}
	openDropdown(t, state)

	rect := dropdownRect()
	rowH := DefaultTheme().Row.Height
	// Second row of the popup that hangs under the button.
	pos := math2d.Vec2{X: rect.Center().X, Y: rect.Max.Y + rowH*1.5}
	next, changed := clickAt(state, pos).dropdownInputIn("filter", rect, len(dropdownLabels()), 0, SpaceRef)

	if next != 1 || !changed {
		t.Fatalf("DropdownInput() = (%d, %v), want (1, true)", next, changed)
	}
	if state.open[HashID("filter")] {
		t.Fatal("picking an item left the menu open")
	}
}

func TestDropdownClosesOnClickOutside(t *testing.T) {
	state := &State{}
	openDropdown(t, state)

	next, changed := pressAt(state, math2d.Vec2{X: 380, Y: 380}).
		dropdownInputIn("filter", dropdownRect(), len(dropdownLabels()), 0, SpaceRef)

	if state.open[HashID("filter")] {
		t.Fatal("a press outside did not close the menu")
	}
	if next != 0 || changed {
		t.Fatalf("DropdownInput() = (%d, %v), want the selection untouched", next, changed)
	}
}

// The popup covers whatever was declared under it, so those widgets must not
// answer the click that picked an item — the overlay intercepts as well as it
// paints.
func TestOpenPopupBlocksWidgetsBeneath(t *testing.T) {
	state := &State{}
	openDropdown(t, state)

	rect := dropdownRect()
	pos := math2d.Vec2{X: rect.Center().X, Y: rect.Max.Y + DefaultTheme().Row.Height/2}
	under := math2d.NewRect(math2d.Vec2{X: 0, Y: 0}, math2d.Vec2{X: 400, Y: 400})

	if clickAt(state, pos).clickedIn("beneath", under, SpaceRef) {
		t.Fatal("a widget under the popup answered the popup's click")
	}
}

func TestDropdownDrawsPopupAboveEveryWidget(t *testing.T) {
	state := &State{}
	openDropdown(t, state)

	cam := testCamera()
	store := AnimStore{}
	u := New(state, math2d.Vec2{}, false, cam, DefaultTheme(), cam.Viewport, &store, 0, 0)
	u.dropdownDrawIn("filter", dropdownRect(), dropdownLabels(), 0, SpaceRef)
	// A widget declared after the dropdown still allocates ordinary layers.
	u.panelIn(math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 10, Y: 10}), SpaceRef)

	popupLayers, buttonLayers := 0, 0
	for _, cmd := range u.Commands() {
		if cmd.Layer() >= LayerOverlay {
			popupLayers++
		} else {
			buttonLayers++
		}
	}
	if popupLayers == 0 {
		t.Fatal("the open popup drew nothing in the overlay block")
	}
	if buttonLayers == 0 {
		t.Fatal("the button drew nothing in the ordinary block")
	}
}

func TestDropdownDrawsSelectedLabel(t *testing.T) {
	cam := testCamera()
	store := AnimStore{}
	u := New(&State{}, math2d.Vec2{}, false, cam, DefaultTheme(), cam.Viewport, &store, 0, 0)
	u.dropdownDrawIn("filter", dropdownRect(), dropdownLabels(), 2, SpaceRef)

	for _, cmd := range u.Commands() {
		if text, ok := cmd.(render.DrawText); ok {
			if text.Text != "Defenders" {
				t.Fatalf("drew %q, want the selected label", text.Text)
			}
			return
		}
	}
	t.Fatal("no label drawn")
}
