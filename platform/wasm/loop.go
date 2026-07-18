//go:build js && wasm

package wasm

import (
	"syscall/js"
	"time"

	"github.com/dhannyell/frameforge/runtime"
)

func (h *host) startLoop() {
	var frameTimer runtime.FrameTimer
	var fpsFrames int
	var fpsElapsed time.Duration
	var fps int

	var cb js.Func
	cb = js.FuncOf(func(this js.Value, args []js.Value) any {
		ts := args[0].Float()
		elapsed := frameTimer.ElapsedMillis(ts)
		inputAtMillis := h.pendingInputAtMillis
		h.pendingInputAtMillis = -1

		_, audioOut := h.driver.RunTick(runtime.TickBatch{Events: h.pending}, elapsed)
		h.pending = h.pending[:0]
		if h.audio != nil {
			h.audio.consume(audioOut)
		}

		frame := h.driver.RenderFrame(elapsed, h.renderer)

		if h.onFrameMetrics != nil {
			inputToSubmit, validInput := inputLatencySince(inputAtMillis)
			if metrics, ready := h.frameMetrics.Observe(elapsed, inputToSubmit, validInput); ready {
				metrics.JSHeapUsed, metrics.JSHeapLimit, metrics.JSHeapSet = browserHeapMetrics()
				if reporter, ok := h.renderer.(gpuBufferReporter); ok {
					metrics.GPUBufferBytes = reporter.particleGPUBufferBytes()
					metrics.GPUBufferSet = true
				}
				if reporter, ok := h.renderer.(gpuTimingReporter); ok {
					metrics.GPUPrepareTime, metrics.GPUSimulateTime, metrics.GPURenderTime, metrics.GPUTimingSet = reporter.particleGPUTimings()
				}
				h.onFrameMetrics(metrics)
			}
		}

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
