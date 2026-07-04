package input

const gamepadCodeStart SourceCode = 2000

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

func isGamepadCode(code SourceCode) bool {
	return code >= GamepadA && code <= GamepadRightStickY
}
