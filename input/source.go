package input

type SourceCode uint16

type Source struct {
	Device DeviceKind
	Code   SourceCode
}

func (s Source) Valid() bool {
	switch s.Device {
	case DeviceKeyboard:
		return IsKeyboardCode(s.Code)
	case DeviceMouse:
		return IsMouseCode(s.Code)
	case DeviceGamepad:
		return IsGamepadCode(s.Code)
	default:
		return false
	}
}
