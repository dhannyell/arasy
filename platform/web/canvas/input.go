package canvas

import "github.com/dhannyell/frameforge/input"

func BrowserKeyboardCode(code string) (input.SourceCode, bool) {
	switch code {
	case "KeyA":
		return input.KeyA, true
	case "KeyB":
		return input.KeyB, true
	case "KeyC":
		return input.KeyC, true
	case "KeyD":
		return input.KeyD, true
	case "KeyE":
		return input.KeyE, true
	case "KeyF":
		return input.KeyF, true
	case "KeyG":
		return input.KeyG, true
	case "KeyH":
		return input.KeyH, true
	case "KeyI":
		return input.KeyI, true
	case "KeyJ":
		return input.KeyJ, true
	case "KeyK":
		return input.KeyK, true
	case "KeyL":
		return input.KeyL, true
	case "KeyM":
		return input.KeyM, true
	case "KeyN":
		return input.KeyN, true
	case "KeyO":
		return input.KeyO, true
	case "KeyP":
		return input.KeyP, true
	case "KeyQ":
		return input.KeyQ, true
	case "KeyR":
		return input.KeyR, true
	case "KeyS":
		return input.KeyS, true
	case "KeyT":
		return input.KeyT, true
	case "KeyU":
		return input.KeyU, true
	case "KeyV":
		return input.KeyV, true
	case "KeyW":
		return input.KeyW, true
	case "KeyX":
		return input.KeyX, true
	case "KeyY":
		return input.KeyY, true
	case "KeyZ":
		return input.KeyZ, true
	case "Space":
		return input.KeySpace, true
	case "Tab":
		return input.KeyTab, true
	case "Enter":
		return input.KeyEnter, true
	case "Escape":
		return input.KeyEscape, true
	case "ArrowUp":
		return input.KeyArrowUp, true
	case "ArrowDown":
		return input.KeyArrowDown, true
	case "ArrowLeft":
		return input.KeyArrowLeft, true
	case "ArrowRight":
		return input.KeyArrowRight, true
	default:
		return input.KeyUnknown, false
	}
}
