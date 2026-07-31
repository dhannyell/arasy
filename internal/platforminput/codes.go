package platforminput

import "github.com/dhannyell/arasy/input"

// KeyboardCode translates a DOM KeyboardEvent.code into a Arasy input code.
func KeyboardCode(code string) (input.SourceCode, bool) {
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
	case "Backspace":
		return input.KeyBackspace, true
	case "Delete":
		return input.KeyDelete, true
	case "Home":
		return input.KeyHome, true
	case "End":
		return input.KeyEnd, true
	case "PageUp":
		return input.KeyPageUp, true
	case "PageDown":
		return input.KeyPageDown, true
	default:
		return 0, false
	}
}

// MouseButtonCode translates a DOM MouseEvent.button into a Arasy input code.
func MouseButtonCode(button int) (input.SourceCode, bool) {
	switch button {
	case 0:
		return input.MouseLeft, true
	case 1:
		return input.MouseMiddle, true
	case 2:
		return input.MouseRight, true
	default:
		return 0, false
	}
}
