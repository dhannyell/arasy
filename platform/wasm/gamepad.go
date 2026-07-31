//go:build js && wasm

package wasm

import (
	"syscall/js"

	"github.com/dhannyell/arasy/internal/platforminput"
)

type gamepadSnapshot = platforminput.GamepadSnapshot

func (h *host) browserGamepadSnapshot() (gamepadSnapshot, bool) {
	navigator := js.Global().Get("navigator")
	getGamepads := navigator.Get("getGamepads")
	if getGamepads.IsUndefined() || getGamepads.IsNull() {
		h.gamepadSelector.Select(nil)
		return gamepadSnapshot{}, false
	}

	gamepads := navigator.Call("getGamepads")
	if cap(h.gamepadSupportScratch) < gamepads.Length() {
		h.gamepadSupportScratch = make([]bool, gamepads.Length())
	} else {
		h.gamepadSupportScratch = h.gamepadSupportScratch[:gamepads.Length()]
		clear(h.gamepadSupportScratch)
	}
	for i := 0; i < gamepads.Length(); i++ {
		gamepad := gamepads.Index(i)
		h.gamepadSupportScratch[i] = browserGamepadSupported(gamepad)
	}
	selected, ok := h.gamepadSelector.Select(h.gamepadSupportScratch)
	if !ok {
		return gamepadSnapshot{}, false
	}
	return browserGamepadState(gamepads.Index(selected)), true
}

func browserGamepadSupported(gamepad js.Value) bool {
	if gamepad.IsNull() || gamepad.IsUndefined() {
		return false
	}
	connected := gamepad.Get("connected")
	if connected.Type() != js.TypeBoolean || !connected.Bool() ||
		gamepad.Get("mapping").String() != "standard" {
		return false
	}
	buttons := gamepad.Get("buttons")
	axes := gamepad.Get("axes")
	return !buttons.IsNull() && !buttons.IsUndefined() &&
		!axes.IsNull() && !axes.IsUndefined()
}

func browserGamepadState(gamepad js.Value) gamepadSnapshot {
	buttons := gamepad.Get("buttons")
	axes := gamepad.Get("axes")
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
	return platforminput.WebStandardGamepadSnapshot(webButtons[:buttonCount], webAxes[:axisCount])
}
