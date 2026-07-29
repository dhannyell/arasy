//go:build windows && amd64

package native

import (
	"github.com/dhannyell/arasy/input"
	"github.com/dhannyell/arasy/internal/platforminput"
	"github.com/go-gl/glfw/v3.4/glfw"
)

func translateKey(key glfw.Key) (input.SourceCode, bool) {
	switch key {
	case glfw.KeyA:
		return input.KeyA, true
	case glfw.KeyB:
		return input.KeyB, true
	case glfw.KeyC:
		return input.KeyC, true
	case glfw.KeyD:
		return input.KeyD, true
	case glfw.KeyE:
		return input.KeyE, true
	case glfw.KeyF:
		return input.KeyF, true
	case glfw.KeyG:
		return input.KeyG, true
	case glfw.KeyH:
		return input.KeyH, true
	case glfw.KeyI:
		return input.KeyI, true
	case glfw.KeyJ:
		return input.KeyJ, true
	case glfw.KeyK:
		return input.KeyK, true
	case glfw.KeyL:
		return input.KeyL, true
	case glfw.KeyM:
		return input.KeyM, true
	case glfw.KeyN:
		return input.KeyN, true
	case glfw.KeyO:
		return input.KeyO, true
	case glfw.KeyP:
		return input.KeyP, true
	case glfw.KeyQ:
		return input.KeyQ, true
	case glfw.KeyR:
		return input.KeyR, true
	case glfw.KeyS:
		return input.KeyS, true
	case glfw.KeyT:
		return input.KeyT, true
	case glfw.KeyU:
		return input.KeyU, true
	case glfw.KeyV:
		return input.KeyV, true
	case glfw.KeyW:
		return input.KeyW, true
	case glfw.KeyX:
		return input.KeyX, true
	case glfw.KeyY:
		return input.KeyY, true
	case glfw.KeyZ:
		return input.KeyZ, true
	case glfw.KeySpace:
		return input.KeySpace, true
	case glfw.KeyTab:
		return input.KeyTab, true
	case glfw.KeyEnter:
		return input.KeyEnter, true
	case glfw.KeyEscape:
		return input.KeyEscape, true
	case glfw.KeyUp:
		return input.KeyArrowUp, true
	case glfw.KeyDown:
		return input.KeyArrowDown, true
	case glfw.KeyLeft:
		return input.KeyArrowLeft, true
	case glfw.KeyRight:
		return input.KeyArrowRight, true
	default:
		return 0, false
	}
}

func translateMouseButton(button glfw.MouseButton) (input.SourceCode, bool) {
	switch button {
	case glfw.MouseButtonLeft:
		return input.MouseLeft, true
	case glfw.MouseButtonRight:
		return input.MouseRight, true
	case glfw.MouseButtonMiddle:
		return input.MouseMiddle, true
	default:
		return 0, false
	}
}

func keyEvent(code input.SourceCode, down bool) input.Event {
	kind := input.EventButtonUp
	if down {
		kind = input.EventButtonDown
	}
	return input.Event{Source: input.Keyboard(code), Kind: kind}
}

func mouseButtonEvent(code input.SourceCode, down bool, x, y float64) input.Event {
	kind := input.EventButtonUp
	if down {
		kind = input.EventButtonDown
	}
	return input.Event{
		Source: input.Source{Device: input.DeviceMouse, Code: code},
		Kind:   kind,
		X:      x,
		Y:      y,
	}
}

func pointerMoveEvent(x, y float64) input.Event {
	return input.Event{
		Source: input.Source{Device: input.DeviceMouse, Code: input.MouseX},
		Kind:   input.EventPointerMove,
		X:      x,
		Y:      y,
	}
}

func wheelEvents(dx, dy, x, y float64) []input.Event {
	events := make([]input.Event, 0, 2)
	if dx != 0 {
		events = append(events, input.Event{
			Source: input.Source{Device: input.DeviceMouse, Code: input.MouseWheelX},
			Kind:   input.EventAxisChanged,
			Value:  dx,
			X:      x,
			Y:      y,
		})
	}
	if dy != 0 {
		events = append(events, input.Event{
			Source: input.Source{Device: input.DeviceMouse, Code: input.MouseWheelY},
			Kind:   input.EventAxisChanged,
			Value:  -dy,
			X:      x,
			Y:      y,
		})
	}
	return events
}

func pointerLeaveEvent() input.Event {
	return input.Event{
		Source: input.Source{Device: input.DeviceMouse, Code: input.MouseUnknown},
		Kind:   input.EventPointerLeave,
	}
}

type gamepadSnapshot = platforminput.GamepadSnapshot

func gamepadEvents(previous, current gamepadSnapshot) []input.Event {
	return platforminput.GamepadEvents(previous, current)
}

func scaleCursor(x, y float64, winW, winH, fbW, fbH int) (float64, float64) {
	if winW == 0 || winH == 0 {
		return 0, 0
	}
	return x * float64(fbW) / float64(winW), y * float64(fbH) / float64(winH)
}
