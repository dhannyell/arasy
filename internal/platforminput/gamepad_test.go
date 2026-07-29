package platforminput_test

import (
	"testing"

	"github.com/dhannyell/arasy/input"
	"github.com/dhannyell/arasy/internal/platforminput"
)

func TestGamepadEventsUseSharedButtonAxisMapping(t *testing.T) {
	current := platforminput.GamepadSnapshot{}
	current.Buttons[0] = true
	current.Axes[0] = .75
	current.Axes[4] = .5

	events := platforminput.GamepadEvents(platforminput.GamepadSnapshot{}, current)
	if len(events) != 3 {
		t.Fatalf("events = %d, want 3", len(events))
	}
	if events[0].Source.Code != input.GamepadA || events[0].Kind != input.EventButtonDown {
		t.Fatalf("button event = %+v", events[0])
	}
	if events[1].Source.Code != input.GamepadLeftStickX || events[1].Value != .75 {
		t.Fatalf("stick event = %+v", events[1])
	}
	if events[2].Source.Code != input.GamepadLeftTrigger || events[2].Value != .5 {
		t.Fatalf("trigger event = %+v", events[2])
	}
}

func TestGamepadEventsReleaseOnDisconnect(t *testing.T) {
	previous := platforminput.GamepadSnapshot{Buttons: [8]bool{true}, Axes: [6]float64{.5}}
	events := platforminput.GamepadEvents(previous, platforminput.GamepadSnapshot{})
	if len(events) != 2 {
		t.Fatalf("disconnect events = %d, want button and axis release", len(events))
	}
	if events[0].Kind != input.EventButtonUp || events[1].Kind != input.EventAxisChanged || events[1].Value != 0 {
		t.Fatalf("disconnect events = %+v", events)
	}
}
