//go:build js && wasm

package wasm

import (
	"syscall/js"

	"github.com/dhannyell/arasy/internal/platforminput"
)

type gamepadSnapshot = platforminput.GamepadSnapshot

func browserGamepadSnapshot() (gamepadSnapshot, bool) {
	navigator := js.Global().Get("navigator")
	getGamepads := navigator.Get("getGamepads")
	if getGamepads.IsUndefined() || getGamepads.IsNull() {
		return gamepadSnapshot{}, false
	}

	gamepads := navigator.Call("getGamepads")
	for i := 0; i < gamepads.Length(); i++ {
		gamepad := gamepads.Index(i)
		if gamepad.IsNull() || gamepad.IsUndefined() {
			continue
		}
		connected := gamepad.Get("connected")
		if !connected.IsUndefined() && !connected.IsNull() && !connected.Bool() {
			continue
		}
		return browserGamepadState(gamepad)
	}
	return gamepadSnapshot{}, false
}

func browserGamepadState(gamepad js.Value) (gamepadSnapshot, bool) {
	buttons := gamepad.Get("buttons")
	axes := gamepad.Get("axes")
	if buttons.IsNull() || buttons.IsUndefined() || axes.IsNull() || axes.IsUndefined() {
		return gamepadSnapshot{}, false
	}

	state := gamepadSnapshot{}
	for i := range state.Buttons {
		if i >= buttons.Length() {
			break
		}
		button := buttons.Index(i)
		state.Buttons[i] = button.Get("pressed").Bool()
		if i == 6 {
			state.Axes[4] = button.Get("value").Float()
		}
		if i == 7 {
			state.Axes[5] = button.Get("value").Float()
		}
	}
	for i := 0; i < 4 && i < axes.Length(); i++ {
		state.Axes[i] = axes.Index(i).Float()
	}
	return state, true
}
