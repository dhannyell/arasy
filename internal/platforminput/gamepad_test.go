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
	previous := platforminput.GamepadSnapshot{Buttons: [15]bool{true}, Axes: [6]float64{.5}}
	events := platforminput.GamepadEvents(previous, platforminput.GamepadSnapshot{})
	if len(events) != 2 {
		t.Fatalf("disconnect events = %d, want button and axis release", len(events))
	}
	if events[0].Kind != input.EventButtonUp || events[1].Kind != input.EventAxisChanged || events[1].Value != 0 {
		t.Fatalf("disconnect events = %+v", events)
	}
}

func TestWebStandardGamepadSnapshotMapsButtonsAndTriggers(t *testing.T) {
	mappings := []struct {
		name      string
		web       int
		canonical int
	}{
		{name: "A", web: 0, canonical: 0},
		{name: "B", web: 1, canonical: 1},
		{name: "X", web: 2, canonical: 2},
		{name: "Y", web: 3, canonical: 3},
		{name: "left bumper", web: 4, canonical: 4},
		{name: "right bumper", web: 5, canonical: 5},
		{name: "Back", web: 8, canonical: 6},
		{name: "Start", web: 9, canonical: 7},
		{name: "Guide", web: 16, canonical: 8},
		{name: "left thumb", web: 10, canonical: 9},
		{name: "right thumb", web: 11, canonical: 10},
		{name: "d-pad up", web: 12, canonical: 11},
		{name: "d-pad right", web: 15, canonical: 12},
		{name: "d-pad down", web: 13, canonical: 13},
		{name: "d-pad left", web: 14, canonical: 14},
	}
	for _, mapping := range mappings {
		t.Run(mapping.name, func(t *testing.T) {
			buttons := make([]platforminput.WebGamepadButton, 17)
			buttons[mapping.web].Pressed = true

			got := platforminput.WebStandardGamepadSnapshot(buttons, nil)

			for i, pressed := range got.Buttons {
				want := i == mapping.canonical
				if pressed != want {
					t.Fatalf("canonical button %d = %v, want %v for W3C button %d", i, pressed, want, mapping.web)
				}
			}
		})
	}

	buttons := make([]platforminput.WebGamepadButton, 8)
	buttons[6].Value = .25
	buttons[7].Value = .75
	axes := []float64{-.5, .5, -.75, .75}

	got := platforminput.WebStandardGamepadSnapshot(buttons, axes)

	if got.Axes != [6]float64{-.5, .5, -.75, .75, .25, .75} {
		t.Fatalf("axes = %v, want signed sticks and unit triggers", got.Axes)
	}
}

func TestWebStandardGamepadSnapshotDoesNotAliasTriggersToBackStart(t *testing.T) {
	buttons := make([]platforminput.WebGamepadButton, 8)
	buttons[6] = platforminput.WebGamepadButton{Pressed: true, Value: 1}
	buttons[7] = platforminput.WebGamepadButton{Pressed: true, Value: 1}

	got := platforminput.WebStandardGamepadSnapshot(buttons, nil)

	if got.Buttons[6] || got.Buttons[7] {
		t.Fatalf("Back/Start = %v/%v, want false for trigger-only input", got.Buttons[6], got.Buttons[7])
	}
	if got.Axes[4] != 1 || got.Axes[5] != 1 {
		t.Fatalf("triggers = %v/%v, want 1/1", got.Axes[4], got.Axes[5])
	}
}

func TestGLFWGamepadSnapshotNormalizesTriggers(t *testing.T) {
	buttons := [15]bool{true}
	tests := []struct {
		name    string
		trigger float64
		want    float64
	}{
		{name: "below range", trigger: -2, want: 0},
		{name: "rest", trigger: -1, want: 0},
		{name: "midpoint", trigger: 0, want: .5},
		{name: "maximum", trigger: 1, want: 1},
		{name: "above range", trigger: 2, want: 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			axes := [6]float64{-.75, .75, -.25, .25, test.trigger, test.trigger}
			got := platforminput.GLFWGamepadSnapshot(buttons, axes)
			if got.Buttons != buttons {
				t.Fatalf("buttons = %v, want %v", got.Buttons, buttons)
			}
			for i := 0; i < 4; i++ {
				if got.Axes[i] != axes[i] {
					t.Fatalf("stick %d = %v, want %v", i, got.Axes[i], axes[i])
				}
			}
			if got.Axes[4] != test.want || got.Axes[5] != test.want {
				t.Fatalf("triggers = %v/%v, want %v", got.Axes[4], got.Axes[5], test.want)
			}
		})
	}
}
