//go:build js && wasm

package wasm

import (
	"syscall/js"

	"github.com/dhannyell/frameforge/platform/web/browserinput"
)

func (h *host) registerInputListeners() {
	window := js.Global()
	canvas := h.canvas

	addWindowListener := func(name string, fn js.Func) {
		h.inputCbs = append(h.inputCbs, fn)
		window.Call("addEventListener", name, fn)
	}

	addCanvasListener := func(name string, fn js.Func) {
		h.inputCbs = append(h.inputCbs, fn)
		canvas.Call("addEventListener", name, fn)
	}

	addWindowListener("keydown", js.FuncOf(func(this js.Value, args []js.Value) any {
		event := args[0]
		if h.handleOverlayToggleKey(event) {
			return nil
		}
		if isEditableTarget(event.Get("target")) {
			return nil
		}
		if event.Get("repeat").Bool() {
			return nil
		}

		keyEvent, ok := browserinput.KeyEvent(event.Get("code").String(), true)
		if !ok {
			return nil
		}

		event.Call("preventDefault")
		h.pending = append(h.pending, keyEvent)
		return nil
	}))

	addWindowListener("keyup", js.FuncOf(func(this js.Value, args []js.Value) any {
		event := args[0]
		if isEditableTarget(event.Get("target")) {
			return nil
		}

		keyEvent, ok := browserinput.KeyEvent(event.Get("code").String(), false)
		if !ok {
			return nil
		}

		event.Call("preventDefault")
		h.pending = append(h.pending, keyEvent)
		return nil
	}))

	addCanvasListener("mousemove", js.FuncOf(func(this js.Value, args []js.Value) any {
		x, y := h.canvasXY(args[0])
		h.pending = append(h.pending, browserinput.MouseMoveEvent(x, y))
		return nil
	}))

	addCanvasListener("mousedown", js.FuncOf(func(this js.Value, args []js.Value) any {
		event := args[0]
		x, y := h.canvasXY(event)

		mouseEvent, ok := browserinput.MouseButtonEvent(event.Get("button").Int(), true, x, y)
		if !ok {
			return nil
		}
		h.pending = append(h.pending, mouseEvent)
		return nil
	}))

	addCanvasListener("mouseup", js.FuncOf(func(this js.Value, args []js.Value) any {
		event := args[0]
		x, y := h.canvasXY(event)

		mouseEvent, ok := browserinput.MouseButtonEvent(event.Get("button").Int(), false, x, y)
		if !ok {
			return nil
		}
		h.pending = append(h.pending, mouseEvent)
		return nil
	}))

	addCanvasListener("mouseleave", js.FuncOf(func(this js.Value, args []js.Value) any {
		h.pending = append(h.pending, browserinput.PointerLeaveEvent())
		return nil
	}))

	addCanvasListener("wheel", js.FuncOf(func(this js.Value, args []js.Value) any {
		event := args[0]
		event.Call("preventDefault")

		x, y := h.canvasXY(event)
		dx := event.Get("deltaX").Float()
		dy := event.Get("deltaY").Float()

		wheelEvents := browserinput.WheelEvent(dx, dy)
		for i := range wheelEvents {
			wheelEvents[i].X = x
			wheelEvents[i].Y = y
		}
		h.pending = append(h.pending, wheelEvents...)

		return nil
	}))
}

func (h *host) handleOverlayToggleKey(event js.Value) bool {
	if h.debugPanel == nil {
		return false
	}
	if isEditableTarget(event.Get("target")) {
		return false
	}

	code := event.Get("code").String()
	key := event.Get("key").String()
	if code != "Backquote" && key != "`" && key != "~" {
		return false
	}

	event.Call("preventDefault")
	event.Call("stopPropagation")
	if event.Get("repeat").Bool() {
		return true
	}
	h.debugPanel.setVisible(!h.debugPanel.visible)
	return true
}

func isEditableTarget(target js.Value) bool {
	if target.IsNull() || target.IsUndefined() {
		return false
	}
	contentEditable := target.Get("isContentEditable")
	if !contentEditable.IsNull() && !contentEditable.IsUndefined() && contentEditable.Bool() {
		return true
	}
	tag := target.Get("tagName")
	if tag.IsNull() || tag.IsUndefined() {
		return false
	}
	switch tag.String() {
	case "INPUT", "TEXTAREA", "SELECT":
		return true
	default:
		return false
	}
}

func (h *host) canvasXY(event js.Value) (float64, float64) {
	rect := h.canvas.Call("getBoundingClientRect")

	rectW := rect.Get("width").Float()
	rectH := rect.Get("height").Float()
	if rectW == 0 || rectH == 0 {
		return 0, 0
	}

	x := (event.Get("clientX").Float() - rect.Get("left").Float()) *
		h.canvas.Get("width").Float() / rectW
	y := (event.Get("clientY").Float() - rect.Get("top").Float()) *
		h.canvas.Get("height").Float() / rectH

	return x, y
}
