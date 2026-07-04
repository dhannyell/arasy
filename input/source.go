package input

type SourceCode uint16

type Source struct {
	Device DeviceKind
	Code   SourceCode
}

func (s Source) Valid() bool {
	switch s.Device {
	case DeviceKeyboard:
		return isKeyboardCode(s.Code)
	case DeviceMouse:
		return isMouseCode(s.Code)
	case DeviceGamepad:
		return isGamepadCode(s.Code)
	default:
		return false
	}
}
