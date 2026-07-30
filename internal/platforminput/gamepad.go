// Package platforminput contains shared host-side input translation helpers.
package platforminput

import "github.com/dhannyell/arasy/input"

// GamepadSnapshot is the canonical state shared by the native and browser
// polling adapters. Its button order matches GLFW's standard gamepad layout;
// axes 4 and 5 carry trigger values normalized to [0, 1].
type GamepadSnapshot struct {
	Buttons [15]bool
	Axes    [6]float64
}

// WebGamepadButton is the host-independent portion of a W3C GamepadButton.
type WebGamepadButton struct {
	Pressed bool
	Value   float64
}

// WebStandardGamepadSnapshot maps the W3C standard layout into the canonical
// snapshot. W3C buttons 6 and 7 are analog triggers, not Back and Start.
func WebStandardGamepadSnapshot(buttons []WebGamepadButton, axes []float64) GamepadSnapshot {
	var snapshot GamepadSnapshot
	webToCanonical := [...]struct {
		web, canonical int
	}{
		{0, 0},   // A
		{1, 1},   // B
		{2, 2},   // X
		{3, 3},   // Y
		{4, 4},   // left bumper
		{5, 5},   // right bumper
		{8, 6},   // Back
		{9, 7},   // Start
		{16, 8},  // Guide
		{10, 9},  // left thumb
		{11, 10}, // right thumb
		{12, 11}, // d-pad up
		{15, 12}, // d-pad right
		{13, 13}, // d-pad down
		{14, 14}, // d-pad left
	}
	for _, mapping := range webToCanonical {
		if mapping.web < len(buttons) {
			snapshot.Buttons[mapping.canonical] = buttons[mapping.web].Pressed
		}
	}
	for i := 0; i < 4 && i < len(axes); i++ {
		snapshot.Axes[i] = axes[i]
	}
	if len(buttons) > 6 {
		snapshot.Axes[4] = clampUnit(buttons[6].Value)
	}
	if len(buttons) > 7 {
		snapshot.Axes[5] = clampUnit(buttons[7].Value)
	}
	return snapshot
}

// GLFWGamepadSnapshot normalizes GLFW's signed trigger axes while preserving
// its canonical button order and signed stick axes.
func GLFWGamepadSnapshot(buttons [15]bool, axes [6]float64) GamepadSnapshot {
	snapshot := GamepadSnapshot{Buttons: buttons, Axes: axes}
	snapshot.Axes[4] = clampUnit((axes[4] + 1) / 2)
	snapshot.Axes[5] = clampUnit((axes[5] + 1) / 2)
	return snapshot
}

func clampUnit(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}

var gamepadButtonCodes = [...]input.SourceCode{
	input.GamepadA,
	input.GamepadB,
	input.GamepadX,
	input.GamepadY,
	input.GamepadLeftBumper,
	input.GamepadRightBumper,
	input.GamepadBack,
	input.GamepadStart,
	input.GamepadGuide,
	input.GamepadLeftThumb,
	input.GamepadRightThumb,
	input.GamepadDpadUp,
	input.GamepadDpadRight,
	input.GamepadDpadDown,
	input.GamepadDpadLeft,
}

var gamepadAxisCodes = [...]input.SourceCode{
	input.GamepadLeftStickX,
	input.GamepadLeftStickY,
	input.GamepadRightStickX,
	input.GamepadRightStickY,
	input.GamepadLeftTrigger,
	input.GamepadRightTrigger,
}

// GamepadEvents converts two normalized samples into deterministic input
// events. A zero current sample therefore also cleanly releases a disconnect.
func GamepadEvents(previous, current GamepadSnapshot) []input.Event {
	events := make([]input.Event, 0, len(gamepadButtonCodes)+len(gamepadAxisCodes))
	for i, code := range gamepadButtonCodes {
		if previous.Buttons[i] == current.Buttons[i] {
			continue
		}
		kind := input.EventButtonUp
		if current.Buttons[i] {
			kind = input.EventButtonDown
		}
		events = append(events, input.Event{
			Source: input.Source{Device: input.DeviceGamepad, Code: code},
			Kind:   kind,
		})
	}
	for i, code := range gamepadAxisCodes {
		value := input.NormalizeGamepadAxis(current.Axes[i])
		if !input.GamepadAxisChanged(previous.Axes[i], value) {
			continue
		}
		events = append(events, input.Event{
			Source: input.Source{Device: input.DeviceGamepad, Code: code},
			Kind:   input.EventAxisChanged,
			Value:  value,
		})
	}
	return events
}
