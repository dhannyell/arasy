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

type textureUpdater interface {
	UpdateTextures(updates []runtime.TextureUpdate)
}

func (h *host) startLoop() {
	var fpsFrames int
	var fpsElapsed time.Duration
	var fps int

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
		if h.audio != nil {
			h.audio.consume(h.driver.ConsumeAudio())
		}

		frame := h.driver.Frame(elapsed)
		if updater, ok := h.renderer.(textureUpdater); ok {
			updater.UpdateTextures(h.driver.ConsumeTextureUpdates())
		}
		h.renderer.Render(frame)

		fpsFrames++
		fpsElapsed += elapsed
		if fpsElapsed >= time.Second {
			fps = fpsFrames
			fpsFrames = 0
			fpsElapsed = 0
		}

		state := h.driver.State()
		if h.debugPanel != nil {
			h.debugPanel.sync(state, fps, len(frame.Commands))
		}

		js.Global().Call("requestAnimationFrame", cb)
		return nil
	})
	h.loopCb = cb
	js.Global().Call("requestAnimationFrame", cb)
}
