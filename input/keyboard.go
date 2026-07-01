package input

const (
	KeyUnknown SourceCode = iota

	KeyA
	KeyB
	KeyC
	KeyD
	KeyE
	KeyF
	KeyG
	KeyH
	KeyI
	KeyJ
	KeyK
	KeyL
	KeyM
	KeyN
	KeyO
	KeyP
	KeyQ
	KeyR
	KeyS
	KeyT
	KeyU
	KeyV
	KeyW
	KeyX
	KeyY
	KeyZ

	KeySpace
	KeyEnter
	KeyEscape

	KeyArrowUp
	KeyArrowDown
	KeyArrowLeft
	KeyArrowRight
)

func Keyboard(code SourceCode) Source {
	return Source{
		Device: DeviceKeyboard,
		Code:   code,
	}
}

func IsKeyboardCode(code SourceCode) bool {
	return code >= KeyA && code <= KeyArrowRight
}
