package input

const mouseCodeStart SourceCode = 1000

const (
	MouseUnknown SourceCode = mouseCodeStart + iota
	MouseLeft
	MouseRight
	MouseMiddle
	MouseX
	MouseY
	MouseWheelX
	MouseWheelY
)

func IsMouseCode(code SourceCode) bool {
	return code >= MouseUnknown && code <= MouseWheelY
}
