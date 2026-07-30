package platforminput

import "github.com/dhannyell/arasy/input"

// KeyEvent converts a DOM KeyboardEvent.code and button direction into a
// validated Arasy event.
func KeyEvent(code string, down bool) (input.Event, bool) {
	key, ok := KeyboardCode(code)
	if !ok {
		return input.Event{}, false
	}
	kind := input.EventButtonUp
	if down {
		kind = input.EventButtonDown
	}
	event := input.Event{
		Source: input.Source{Device: input.DeviceKeyboard, Code: key},
		Kind:   kind,
	}
	return event, event.Valid()
}

// MouseButtonEvent converts a DOM MouseEvent.button value into a validated
// pointer button event.
func MouseButtonEvent(button int, down bool, x, y float64) (input.Event, bool) {
	code, ok := MouseButtonCode(button)
	if !ok {
		return input.Event{}, false
	}
	kind := input.EventButtonUp
	if down {
		kind = input.EventButtonDown
	}
	event := input.Event{
		Source: input.Source{Device: input.DeviceMouse, Code: code},
		Kind:   kind,
		X:      x,
		Y:      y,
	}
	return event, event.Valid()
}

// MouseButtonEventCode is the equivalent constructor for the string codes
// used by the canvas debug host's JSON protocol.
func MouseButtonEventCode(code string, down bool, x, y float64) (input.Event, bool) {
	var button int
	switch code {
	case "Left":
		button = 0
	case "Middle":
		button = 1
	case "Right":
		button = 2
	default:
		return input.Event{}, false
	}
	return MouseButtonEvent(button, down, x, y)
}

func MouseMoveEvent(x, y float64) input.Event {
	return input.Event{
		Source: input.Source{Device: input.DeviceMouse, Code: input.MouseX},
		Kind:   input.EventPointerMove,
		X:      x,
		Y:      y,
	}
}

// WheelEvent emits separate events for non-zero horizontal and vertical
// wheel axes. Hosts may add the pointer coordinates after construction.
func WheelEvent(dx, dy float64) []input.Event {
	events := make([]input.Event, 0, 2)
	if dx != 0 {
		events = append(events, input.Event{
			Source: input.Source{Device: input.DeviceMouse, Code: input.MouseWheelX},
			Kind:   input.EventAxisChanged,
			Value:  dx,
		})
	}
	if dy != 0 {
		events = append(events, input.Event{
			Source: input.Source{Device: input.DeviceMouse, Code: input.MouseWheelY},
			Kind:   input.EventAxisChanged,
			Value:  dy,
		})
	}
	return events
}

func PointerLeaveEvent() input.Event {
	return input.Event{
		Source: input.Source{Device: input.DeviceMouse, Code: input.MouseUnknown},
		Kind:   input.EventPointerLeave,
	}
}
