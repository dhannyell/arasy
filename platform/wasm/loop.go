//go:build js && wasm

package wasm

import (
	"syscall/js"
	"time"

	"github.com/dhannyell/arasy/runtime"
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

		sampleCPU := h.onFrameMetrics != nil
		var tickStart float64
		if sampleCPU {
			tickStart = performanceNowMillis()
		}
		_, audioOut := h.driver.RunTick(runtime.TickBatch{Events: h.pending}, elapsed)
		h.pending = h.pending[:0]
		if h.audio != nil {
			h.audio.setAudioContextState(h.driver.State().Paused)
			h.audio.consume(audioOut)
		}
		var tickEnd float64
		if sampleCPU {
			tickEnd = performanceNowMillis()
		}

		frame := h.driver.RenderFrame(elapsed, h.renderer)

		if h.onFrameMetrics != nil {
			renderEnd := performanceNowMillis()
			inputToSubmit, validInput := inputLatencySince(inputAtMillis)
			if metrics, ready := h.frameMetrics.Observe(elapsed, inputToSubmit, validInput); ready {
				metrics.CPUTickTime = durationFromMillis(tickEnd - tickStart)
				metrics.CPURenderTime = durationFromMillis(renderEnd - tickEnd)
				metrics.CPUTimingSet = true
				if reporter, ok := h.renderer.(cpuTimingReporter); ok {
					metrics.CPUTessTime, metrics.CPUUploadTime, metrics.CPUEncodeTime, metrics.CPUBreakdownSet = reporter.renderCPUTimings()
				}
				metrics.JSHeapUsed, metrics.JSHeapLimit, metrics.JSHeapSet = browserHeapMetrics()
				if reporter, ok := h.renderer.(gpuBufferReporter); ok {
					metrics.GPUBufferBytes = reporter.particleGPUBufferBytes()
					metrics.GPUBufferSet = true
				}
				if reporter, ok := h.renderer.(gpuTimingReporter); ok {
					metrics.GPUPrepareTime, metrics.GPUSimulateTime, metrics.GPURenderTime, metrics.GPUTimingSet = reporter.particleGPUTimings()
				}
				if reporter, ok := h.renderer.(retainedBatchReporter); ok {
					metrics.Retained, metrics.RetainedSet = reporter.retainedBatchMetrics()
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
