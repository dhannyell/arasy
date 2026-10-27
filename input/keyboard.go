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
	KeyTab
	KeyEnter
	KeyEscape

	KeyArrowUp
	KeyArrowDown
	KeyArrowLeft
	KeyArrowRight

	// Editing keys. They are ordinary buttons like every key above — text
	// itself travels as EventText — but a text field needs them named, and
	// both hosts can produce them.
	KeyBackspace
	KeyDelete
	KeyHome
	KeyEnd
	KeyPageUp
	KeyPageDown

	// Bracket keys, the usual pair for stepping a value up and down.
	KeyLeftBracket
	KeyRightBracket
)

// keyCodeCount bounds the keyboard catalogue, so a per-tick snapshot of held
// keys is a fixed array rather than a map: cheap to clone into a rewind
// snapshot and iterated in a fixed order.
const keyCodeCount = int(KeyRightBracket) + 1

func Keyboard(code SourceCode) Source {
	return Source{
		Device: DeviceKeyboard,
		Code:   code,
	}
}

func isKeyboardCode(code SourceCode) bool {
	return code >= KeyA && code <= KeyRightBracket
}
