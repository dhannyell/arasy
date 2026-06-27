package input

type DeviceKind uint8

const (
	DeviceUnknown DeviceKind = iota
	DeviceKeyboard
	DeviceMouse
	DeviceGamepad
)
