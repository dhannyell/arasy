package runtime

import (
	"github.com/dhannyell/arasy/input"
)

// debugGamepadCodeCount spans the slot-zero gamepad catalogue, so the mirror
// below is a fixed array rather than a map: the overlay iterates it in a
// stable order every frame instead of shuffling its lines around.
const debugGamepadCodeCount = int(input.GamepadRightStickY-input.GamepadA) + 1

// debugGamepadState mirrors the raw gamepad events the driver applied, which
// nothing else keeps: the Mapper turns them into what the game means by them
// and discards the rest, and that is exactly the layer a "why is my binding
// not firing" question needs to see underneath.
//
// It is display-only. It stays out of snapshots and the checksum on purpose,
// so after a rewind the overlay reports what the pad is doing now rather than
// what it was doing at the tick being re-entered.
type debugGamepadState struct {
	buttons [debugGamepadCodeCount]bool
	axes    [debugGamepadCodeCount]float64
	// seen records that a gamepad event ever arrived, which distinguishes a
	// pad at rest from no pad at all.
	seen bool
}

func (s *debugGamepadState) apply(event input.Event) {
	// Only slot zero is emitted by the built-in hosts today; higher slots are
	// reserved in the code space and simply do not appear here yet.
	index := int(event.Source.Code) - int(input.GamepadA)
	if index < 0 || index >= debugGamepadCodeCount {
		return
	}
	s.seen = true

	switch event.Kind {
	case input.EventButtonDown:
		s.buttons[index] = true
	case input.EventButtonUp:
		s.buttons[index] = false
	case input.EventAxisChanged:
		s.axes[index] = event.Value
	}
}

func (s *debugGamepadState) reset() { *s = debugGamepadState{} }

// pressed lists the held buttons by name, in catalogue order.
func (s *debugGamepadState) pressed() []string {
	var names []string
	for index, down := range s.buttons {
		if down {
			names = append(names, debugGamepadName(input.GamepadA+input.SourceCode(index)))
		}
	}
	return names
}

func debugGamepadName(code input.SourceCode) string {
	switch code {
	case input.GamepadA:
		return "A"
	case input.GamepadB:
		return "B"
	case input.GamepadX:
		return "X"
	case input.GamepadY:
		return "Y"
	case input.GamepadLeftBumper:
		return "LB"
	case input.GamepadRightBumper:
		return "RB"
	case input.GamepadBack:
		return "Back"
	case input.GamepadStart:
		return "Start"
	case input.GamepadGuide:
		return "Guide"
	case input.GamepadLeftThumb:
		return "LS"
	case input.GamepadRightThumb:
		return "RS"
	case input.GamepadDpadUp:
		return "Up"
	case input.GamepadDpadRight:
		return "Right"
	case input.GamepadDpadDown:
		return "Down"
	case input.GamepadDpadLeft:
		return "Left"
	case input.GamepadLeftTrigger:
		return "LT"
	case input.GamepadRightTrigger:
		return "RT"
	case input.GamepadLeftStickX:
		return "LX"
	case input.GamepadLeftStickY:
		return "LY"
	case input.GamepadRightStickX:
		return "RX"
	case input.GamepadRightStickY:
		return "RY"
	default:
		return "Pad?"
	}
}

func debugDeviceName(device input.DeviceKind) string {
	switch device {
	case input.DeviceKeyboard:
		return "keyboard"
	case input.DeviceMouse:
		return "mouse"
	case input.DeviceGamepad:
		return "gamepad"
	default:
		return "none"
	}
}

// debugKeyName names a key for the overlay. Letters derive from the
// catalogue's own ordering rather than twenty-six switch arms.
func debugKeyName(code input.SourceCode) string {
	if code >= input.KeyA && code <= input.KeyZ {
		return string(rune('A' + code - input.KeyA))
	}
	switch code {
	case input.KeySpace:
		return "Space"
	case input.KeyTab:
		return "Tab"
	case input.KeyEnter:
		return "Enter"
	case input.KeyEscape:
		return "Esc"
	case input.KeyArrowUp:
		return "Up"
	case input.KeyArrowDown:
		return "Down"
	case input.KeyArrowLeft:
		return "Left"
	case input.KeyArrowRight:
		return "Right"
	case input.KeyBackspace:
		return "Backspace"
	case input.KeyDelete:
		return "Delete"
	case input.KeyHome:
		return "Home"
	case input.KeyEnd:
		return "End"
	case input.KeyPageUp:
		return "PageUp"
	case input.KeyPageDown:
		return "PageDown"
	default:
		return "Key?"
	}
}
