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
	// Only the W3C standard mapping has portable button semantics. Treat an
	// unmapped device as unavailable instead of emitting mislabeled input.
	if gamepad.Get("mapping").String() != "standard" {
		return gamepadSnapshot{}, false
	}
	buttons := gamepad.Get("buttons")
	axes := gamepad.Get("axes")
	if buttons.IsNull() || buttons.IsUndefined() || axes.IsNull() || axes.IsUndefined() {
		return gamepadSnapshot{}, false
	}

	var webButtons [17]platforminput.WebGamepadButton
	buttonCount := min(buttons.Length(), len(webButtons))
	for i := 0; i < buttonCount; i++ {
		button := buttons.Index(i)
		webButtons[i] = platforminput.WebGamepadButton{
			Pressed: button.Get("pressed").Bool(),
			Value:   button.Get("value").Float(),
		}
	}
	var webAxes [4]float64
	axisCount := min(axes.Length(), len(webAxes))
	for i := 0; i < axisCount; i++ {
		webAxes[i] = axes.Index(i).Float()
	}
	return platforminput.WebStandardGamepadSnapshot(webButtons[:buttonCount], webAxes[:axisCount]), true
}
