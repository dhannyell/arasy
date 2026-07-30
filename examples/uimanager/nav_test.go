package uimanager

import (
	"io"
	"log/slog"
	"testing"

	"github.com/dhannyell/arasy/input"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/runtime"
)

func newDriver(t *testing.T) (*Manager, *runtime.Driver) {
	t.Helper()
	manager := &Manager{}
	driver := runtime.NewDriver(manager, slog.New(slog.NewTextHandler(io.Discard, nil)))
	driver.Step()
	return manager, driver
}

func clickNav(d *runtime.Driver, pos math2d.Vec2) {
	d.ApplyInput([]input.Event{
		{Source: input.Source{Device: input.DeviceMouse, Code: input.MouseX}, Kind: input.EventPointerMove, X: pos.X, Y: pos.Y},
		{Source: input.Source{Device: input.DeviceMouse, Code: input.MouseLeft}, Kind: input.EventButtonDown, X: pos.X, Y: pos.Y},
		{Source: input.Source{Device: input.DeviceMouse, Code: input.MouseLeft}, Kind: input.EventButtonUp, X: pos.X, Y: pos.Y},
	})
	d.Step()
}

func pressArrow(d *runtime.Driver, code input.SourceCode) {
	d.ApplyInput([]input.Event{{Source: input.Source{Device: input.DeviceKeyboard, Code: code}, Kind: input.EventButtonDown}})
	d.Step()
	d.ApplyInput([]input.Event{{Source: input.Source{Device: input.DeviceKeyboard, Code: code}, Kind: input.EventButtonUp}})
	d.Step()
}

func TestClickingNavHeaderSwitchesScreen(t *testing.T) {
	manager, driver := newDriver(t)

	clickNav(driver, navItems[ScreenSquad].rect.Center())

	if manager.State.Screen != ScreenSquad {
		t.Fatalf("screen = %v, want %v", manager.State.Screen, ScreenSquad)
	}
	if manager.State.PreviousScreen != ScreenDashboard {
		t.Fatalf("previous screen = %v, want %v so the slide transition has a source",
			manager.State.PreviousScreen, ScreenDashboard)
	}
}

// The gaps between nav entries are dead space, as they were when each entry
// hit-tested itself.
func TestClickingBetweenNavHeadersDoesNothing(t *testing.T) {
	manager, driver := newDriver(t)
	first := navItems[0].rect

	clickNav(driver, math2d.Vec2{X: first.Max.X + navItemGap/2, Y: first.Center().Y})

	if manager.State.Screen != ScreenDashboard {
		t.Fatalf("screen = %v, want the gap click ignored", manager.State.Screen)
	}
}

// The point of keeping the index in ManagerState: pointer and keyboard write
// the same field, so neither can go stale behind the other. A mirrored copy
// inside the UI would drift here.
func TestKeyboardAndPointerShareOneScreenState(t *testing.T) {
	manager, driver := newDriver(t)

	pressArrow(driver, input.KeyArrowRight)
	if manager.State.Screen != ScreenTable {
		t.Fatalf("screen after keyboard = %v, want %v", manager.State.Screen, ScreenTable)
	}

	// Clicking the entry the keyboard just moved to must be a no-op, which is
	// only true if the widget hit-tested against the keyboard's result.
	clickNav(driver, navItems[ScreenTable].rect.Center())
	if manager.State.Screen != ScreenTable {
		t.Fatalf("screen = %v, want it to stay %v", manager.State.Screen, ScreenTable)
	}
	if manager.State.PreviousScreen != ScreenDashboard {
		t.Fatalf("previous screen = %v, want the redundant click to leave it at %v",
			manager.State.PreviousScreen, ScreenDashboard)
	}

	clickNav(driver, navItems[ScreenCalendar].rect.Center())
	if manager.State.Screen != ScreenCalendar || manager.State.PreviousScreen != ScreenTable {
		t.Fatalf("screen/previous = %v/%v, want %v/%v",
			manager.State.Screen, manager.State.PreviousScreen, ScreenCalendar, ScreenTable)
	}
}

// The active screen is game state, so a rewind has to take it back with the
// rest of the simulation.
func TestScreenSelectionSurvivesRewind(t *testing.T) {
	manager, driver := newDriver(t)
	before := driver.State().Tick

	clickNav(driver, navItems[ScreenCalendar].rect.Center())
	if manager.State.Screen != ScreenCalendar {
		t.Fatalf("screen = %v, want %v", manager.State.Screen, ScreenCalendar)
	}

	driver.Control(runtime.ControlCommand{Kind: runtime.ControlScrubToTick, Tick: before})
	driver.Step()

	if manager.State.Screen != ScreenDashboard {
		t.Fatalf("screen after rewind = %v, want %v", manager.State.Screen, ScreenDashboard)
	}
}
