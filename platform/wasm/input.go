//go:build js && wasm

package wasm

import (
	"syscall/js"

	"github.com/dhannyell/arasy/input"
	"github.com/dhannyell/arasy/internal/platforminput"
)

func (h *host) registerInputListeners() {
	window := js.Global()
	canvas := h.canvas
	canvas.Get("style").Set("touchAction", "none")

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

		keyEvent, ok := platforminput.KeyEvent(event.Get("code").String(), true)
		if !ok {
			return nil
		}

		event.Call("preventDefault")
		h.queueInput(keyEvent, event)
		return nil
	}))

	addWindowListener("keyup", js.FuncOf(func(this js.Value, args []js.Value) any {
		event := args[0]
		if isEditableTarget(event.Get("target")) {
			return nil
		}

		keyEvent, ok := platforminput.KeyEvent(event.Get("code").String(), false)
		if !ok {
			return nil
		}

		event.Call("preventDefault")
		h.queueInput(keyEvent, event)
		return nil
	}))

	addCanvasListener("pointermove", js.FuncOf(func(this js.Value, args []js.Value) any {
		event := args[0]
		if event.Get("pointerType").String() == "touch" {
			if !h.touch.Accept(event.Get("pointerId").Int(), platforminput.TouchMove) {
				return nil
			}
		}
		x, y := h.canvasXY(event)
		h.queueInput(platforminput.MouseMoveEvent(x, y), event)
		return nil
	}))

	addCanvasListener("pointerdown", js.FuncOf(func(this js.Value, args []js.Value) any {
		event := args[0]
		if event.Get("pointerType").String() == "touch" {
			if !h.touch.Accept(event.Get("pointerId").Int(), platforminput.TouchBegin) {
				return nil
			}
			event.Call("preventDefault")
			canvas.Call("setPointerCapture", event.Get("pointerId"))
		}
		x, y := h.canvasXY(event)

		mouseEvent, ok := platforminput.MouseButtonEvent(event.Get("button").Int(), true, x, y)
		if !ok {
			return nil
		}
		h.queueInput(mouseEvent, event)
		return nil
	}))

	addCanvasListener("pointerup", js.FuncOf(func(this js.Value, args []js.Value) any {
		event := args[0]
		isTouch := event.Get("pointerType").String() == "touch"
		if isTouch {
			if !h.touch.Accept(event.Get("pointerId").Int(), platforminput.TouchEnd) {
				return nil
			}
			event.Call("preventDefault")
			canvas.Call("releasePointerCapture", event.Get("pointerId"))
		}
		x, y := h.canvasXY(event)

		mouseEvent, ok := platforminput.MouseButtonEvent(event.Get("button").Int(), false, x, y)
		if !ok {
			return nil
		}
		h.queueInput(mouseEvent, event)
		if isTouch {
			// Touch has no hover state: end the pointer frame at the same
			// boundary as the up event while preserving the click edge.
			h.queueInput(platforminput.PointerLeaveEvent(), event)
		}
		return nil
	}))

	addCanvasListener("pointerleave", js.FuncOf(func(this js.Value, args []js.Value) any {
		event := args[0]
		if event.Get("pointerType").String() == "touch" &&
			!h.touch.Accept(event.Get("pointerId").Int(), platforminput.TouchMove) {
			return nil
		}
		h.queueInput(platforminput.PointerLeaveEvent(), event)
		return nil
	}))

	addCanvasListener("pointercancel", js.FuncOf(func(this js.Value, args []js.Value) any {
		event := args[0]
		if event.Get("pointerType").String() == "touch" {
			if !h.touch.Accept(event.Get("pointerId").Int(), platforminput.TouchEnd) {
				return nil
			}
		}
		h.queueInput(platforminput.PointerLeaveEvent(), event)
		return nil
	}))

	addCanvasListener("wheel", js.FuncOf(func(this js.Value, args []js.Value) any {
		event := args[0]
		event.Call("preventDefault")

		x, y := h.canvasXY(event)
		dx := event.Get("deltaX").Float()
		dy := event.Get("deltaY").Float()

		wheelEvents := platforminput.WheelEvent(dx, dy)
		for i := range wheelEvents {
			wheelEvents[i].X = x
			wheelEvents[i].Y = y
		}
		for _, wheelEvent := range wheelEvents {
			h.queueInput(wheelEvent, event)
		}

		return nil
	}))

	addCanvasListener("contextmenu", js.FuncOf(func(this js.Value, args []js.Value) any {
		event := args[0]
		event.Call("preventDefault")
		return nil
	}))
}

func (h *host) queueInput(inputEvent input.Event, browserEvent js.Value) {
	stamp := browserEvent.Get("timeStamp")
	if !stamp.IsUndefined() && !stamp.IsNull() {
		h.noteInputAt(stamp.Float(), 1)
	}
	h.pending = append(h.pending, inputEvent)
}

func (h *host) noteInputAt(millis float64, eventCount int) {
	if h.onFrameMetrics == nil || eventCount == 0 {
		return
	}
	h.pendingInputAtMillis = pendingInputMillis(
		h.pendingInputAtMillis,
		millis,
		performanceNowMillis(),
		eventCount,
	)
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
