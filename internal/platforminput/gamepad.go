// Package platforminput contains shared host-side input translation helpers.
package platforminput

import "github.com/dhannyell/arasy/input"

// GamepadSnapshot is the normalized state shared by the native and browser
// polling adapters. Buttons 6 and 7 are the trigger buttons; axes 4 and 5
// carry their analog values.
type GamepadSnapshot struct {
	Buttons [8]bool
	Axes    [6]float64
}

var gamepadButtonCodes = [...]input.SourceCode{
	input.GamepadA,
	input.GamepadB,
	input.GamepadX,
	input.GamepadY,
	input.GamepadLeftShoulder,
	input.GamepadRightShoulder,
	input.GamepadLeftTrigger,
	input.GamepadRightTrigger,
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
