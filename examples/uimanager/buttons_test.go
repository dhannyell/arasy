package uimanager

import (
	"testing"

	"github.com/dhannyell/arasy/input"
	"github.com/dhannyell/arasy/render"
	"github.com/dhannyell/arasy/runtime"
)

// The engine adds its own Enter -> ActionConfirm binding to every game, and the
// mapper sums the bindings of one action. A demo binding with the opposite
// scale therefore cancels the engine's and leaves ActionConfirm dead, which
// looks exactly like "keyboard activation is not implemented". It is not
// obvious from reading either binding alone, so it gets a test.
func TestEnterReachesActionConfirm(t *testing.T) {
	mapper := input.NewMapper(append(matchdayBindings(),
		input.Binding{Source: input.Keyboard(input.KeyEnter), Action: input.ActionConfirm, Scale: 1}))
	mapper.Apply(input.Event{Source: input.Keyboard(input.KeyEnter), Kind: input.EventButtonDown})

	if state := mapper.State(); !state.Pressed(input.ActionConfirm) {
		t.Fatalf("Enter does not reach ActionConfirm (value = %v); a demo binding is cancelling the engine default",
			state.Value(input.ActionConfirm))
	}
}

// The gamepad reaches the same two UI actions the keyboard does. The engine
// binds only the keyboard by default, so a pad that cannot focus or confirm is
// a missing binding here, not a missing feature in the engine.
func TestGamepadReachesTheUIActions(t *testing.T) {
	cases := map[input.SourceCode]input.Action{
		input.GamepadA:     input.ActionConfirm,
		input.GamepadStart: input.ActionFocusNext,
	}
	for code, action := range cases {
		mapper := input.NewMapper(matchdayBindings())
		mapper.Apply(input.Event{Source: input.Gamepad(code), Kind: input.EventButtonDown})

		state := mapper.State()
		if !state.Pressed(action) {
			t.Fatalf("gamepad code %d does not reach %v", code, action)
		}
	}
}

// Tab focuses the first declared button and Enter runs it — the demo's primary
// action must be reachable without a mouse.
func TestSimulateRoundRunsFromTheKeyboard(t *testing.T) {
	manager, driver := newDriver(t)
	before := manager.State.Round

	pressKey(driver, input.KeyTab)
	pressKey(driver, input.KeyEnter)

	if manager.State.Round == before {
		t.Fatalf("round = %d, want the keyboard to have simulated one", manager.State.Round)
	}
}

func TestCalendarSimulateRunsFromTheKeyboard(t *testing.T) {
	manager, driver := newDriver(t)
	clickNav(driver, navItems[ScreenCalendar].rect.Center())
	before := manager.State.Round

	pressKey(driver, input.KeyTab)
	pressKey(driver, input.KeyEnter)

	if manager.State.Round == before {
		t.Fatalf("round = %d, want the calendar button to answer the keyboard", manager.State.Round)
	}
}

// Focus has to be visible, or keyboard navigation is a feature nobody can see.
// The outline repeats the button's slanted silhouette, so it is a polygon.
func TestFocusedButtonDrawsAnOutline(t *testing.T) {
	manager, driver := newDriver(t)

	outlines := func() int { return countFocusOutlines(driver) }

	if got := outlines(); got != 0 {
		t.Fatalf("outlines = %d with nothing focused, want 0", got)
	}

	pressKey(driver, input.KeyTab)
	driver.Step()

	if got := outlines(); got != 1 {
		t.Fatalf("outlines = %d with the button focused, want 1", got)
	}
	if manager.State.Screen != ScreenDashboard {
		t.Fatalf("screen = %v, want Tab to have left the screen alone", manager.State.Screen)
	}
}

// Focus is UI state the driver owns, so a rewind takes it back with the rest
// of the snapshot — visible here through the outline disappearing again.
func TestButtonFocusRewindsWithTheSnapshot(t *testing.T) {
	_, driver := newDriver(t)
	before := driver.State().Tick

	pressKey(driver, input.KeyTab)
	driver.Step()
	if countFocusOutlines(driver) != 1 {
		t.Fatal("Tab did not focus the button")
	}

	driver.Control(runtime.ControlCommand{Kind: runtime.ControlScrubToTick, Tick: before})
	driver.Step()

	if got := countFocusOutlines(driver); got != 0 {
		t.Fatalf("outlines = %d after rewind, want the focus taken back", got)
	}
}

func countFocusOutlines(driver *runtime.Driver) int {
	count := 0
	for _, cmd := range driver.Frame(0).Commands {
		poly, ok := cmd.(render.DrawPolygon)
		if ok && poly.Stroke == focusColor && poly.StrokeWidth == focusOutlineWidth {
			count++
		}
	}
	return count
}
