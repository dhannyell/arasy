package input

import "math"

const gamepadCodeStart SourceCode = 2000
const gamepadSlotStride SourceCode = 100
const gamepadSlotCount = 16

// GamepadAxisEpsilon is the fixed host-side threshold used to suppress
// analog jitter. It is intentionally not configurable: changing it would
// change the event journal without changing the recorded simulation state.
const GamepadAxisEpsilon = 0.001

// DefaultGamepadDeadzone is the effective deadzone used when a runtime config
// leaves GamepadDeadzone at its zero value.
const DefaultGamepadDeadzone = 0.15

const (
	GamepadUnknown SourceCode = gamepadCodeStart + iota
	GamepadA
	GamepadB
	GamepadX
	GamepadY
	GamepadLeftShoulder
	GamepadRightShoulder
	GamepadLeftTrigger
	GamepadRightTrigger
	GamepadLeftStickX
	GamepadLeftStickY
	GamepadRightStickX
	GamepadRightStickY
)

// Gamepad constructs a gamepad source for slot zero.
func Gamepad(code SourceCode) Source {
	return Source{Device: DeviceGamepad, Code: code}
}

// EffectiveGamepadDeadzone resolves the runtime zero value and rejects
// unusable configuration while keeping the result in [0, 1).
func EffectiveGamepadDeadzone(value float64) float64 {
	if value == 0 || value < 0 || value >= 1 || math.IsNaN(value) {
		return DefaultGamepadDeadzone
	}
	return value
}

func isGamepadCode(code SourceCode) bool {
	if code < gamepadCodeStart || code >= gamepadCodeStart+gamepadSlotStride*gamepadSlotCount {
		return false
	}
	offset := (code - gamepadCodeStart) % gamepadSlotStride
	return offset >= GamepadA-gamepadCodeStart && offset <= GamepadRightStickY-gamepadCodeStart
}

// GamepadSlotCode maps one of the slot-zero gamepad codes into a reserved
// slot range. Only slot zero is emitted by the built-in hosts today, but the
// ranges are reserved so adding local multiplayer does not change Event.
func GamepadSlotCode(slot int, code SourceCode) (SourceCode, bool) {
	if slot < 0 || slot >= gamepadSlotCount || code < GamepadA || code > GamepadRightStickY {
		return 0, false
	}
	return code + SourceCode(slot)*gamepadSlotStride, true
}

// NormalizeGamepadAxis clamps a host sample to the public [-1, 1] range.
func NormalizeGamepadAxis(value float64) float64 {
	if math.IsNaN(value) {
		return 0
	}
	if value < -1 {
		return -1
	}
	if value > 1 {
		return 1
	}
	return value
}

// GamepadAxisChanged reports whether a new normalized sample is worth
// recording after the fixed jitter threshold is applied.
func GamepadAxisChanged(previous, current float64) bool {
	previous = NormalizeGamepadAxis(previous)
	current = NormalizeGamepadAxis(current)
	return math.Abs(current-previous) > GamepadAxisEpsilon
}

// ApplyGamepadDeadzone removes the center deadzone and rescales the remaining
// range back to [-1, 1]. Values exactly on the boundary are treated as zero;
// only values strictly above it reach the action mapper.
func ApplyGamepadDeadzone(value, deadzone float64) float64 {
	value = NormalizeGamepadAxis(value)
	if deadzone <= 0 {
		return value
	}
	if deadzone >= 1 || math.IsNaN(deadzone) {
		return 0
	}
	abs := math.Abs(value)
	if abs <= deadzone {
		return 0
	}
	negative := math.Signbit(value)
	value = (abs - deadzone) / (1 - deadzone)
	if value > 1 {
		value = 1
	}
	if value < 0 {
		return 0
	}
	if negative {
		return -value
	}
	return value
}
