//go:build js && wasm

package wasm

import (
	"syscall/js"
	"time"

	"github.com/dhannyell/frameforge/runtime"
)

// renderer draws one runtime.Frame into the canvas.
type renderer interface {
	Render(frame runtime.Frame)
}

func (h *host) startLoop() {
	var cb js.Func
	cb = js.FuncOf(func(this js.Value, args []js.Value) any {
		ts := args[0].Float()
		elapsed := time.Duration(0)
		if h.hasTimestamp {
			elapsed = time.Duration((ts - h.lastTimestamp) * float64(time.Millisecond))
		}
		h.lastTimestamp = ts
		h.hasTimestamp = true

		h.driver.ApplyInput(h.pending)
		h.pending = h.pending[:0]
		h.driver.AdvanceTime(elapsed)

		frame := h.driver.Frame(elapsed)
		h.renderer.Render(frame)

		if h.panel != nil {
			h.panel.sync(h.driver.State())
		}

		js.Global().Call("requestAnimationFrame", cb)
		return nil
	})
	h.loopCb = cb
	js.Global().Call("requestAnimationFrame", cb)
}
