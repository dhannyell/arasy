//go:build js && wasm

package wasm

import (
	"syscall/js"
	"time"

	"github.com/dhannyell/arasy/input"
	"github.com/dhannyell/arasy/runtime"
)

func (h *host) startLoop() {
	var frameTimer runtime.FrameTimer
	var firstFramePresented bool

	var cb js.Func
	cb = js.FuncOf(func(this js.Value, args []js.Value) any {
		ts := args[0].Float()
		elapsed := frameTimer.ElapsedMillis(ts)
		if events := h.pollGamepad(); len(events) > 0 {
			h.noteInputAt(ts, len(events))
			h.pending = append(h.pending, events...)
		}

		splash := h.driver.RunSplashFrame(runtime.SplashFrameBatch{
			Events: h.pending,
		}, elapsed, h.renderer)
		if splash.Rendered {
			h.pending = h.pending[:0]
			h.pendingInputAtMillis = -1
			if splash.ReadyForHandoff {
				signalFirstFramePresented(&firstFramePresented)
			}
			js.Global().Call("requestAnimationFrame", cb)
			return nil
		}

		inputAtMillis := h.pendingInputAtMillis
		h.pendingInputAtMillis = -1

		// The dev panel reports the same split the metrics callback does, so
		// an app that never asked for metrics still gets a Perf tab.
		sampleCPU := h.onFrameMetrics != nil || h.panelWantsStats()
		var tickStart float64
		if sampleCPU {
			tickStart = performanceNowMillis()
		}
		// The panel sees every event first and answers with the ones the game
		// should still get. Whatever it kept never reaches the driver, so
		// clicking Pause is not part of the session being recorded.
		events := h.pending
		if h.panel != nil {
			events = h.panel.HandleInput(events, h.viewport)
		}
		_, audioOut := h.driver.RunTick(runtime.TickBatch{Events: events}, elapsed)
		h.pending = h.pending[:0]
		if h.audio != nil {
			h.audio.setAudioContextState(h.driver.State().Paused)
			h.audio.consume(audioOut)
		}
		var tickEnd float64
		if sampleCPU {
			tickEnd = performanceNowMillis()
		}

		h.renderFrame(elapsed)
		signalFirstFramePresented(&firstFramePresented)
		if h.panelWantsStats() {
			h.samplePanelStats(tickStart, tickEnd, inputAtMillis)
		}

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

		js.Global().Call("requestAnimationFrame", cb)
		return nil
	})
	h.loopCb = cb
	js.Global().Call("requestAnimationFrame", cb)
}

// renderFrame is runtime.RenderFrame with the dev panel spliced in: the
// panel's commands are appended after the game's, in a layer band above every
// game layer. With the panel closed this is command-for-command the frame the
// driver would have rendered on its own.
func (h *host) renderFrame(elapsed time.Duration) {
	frame := h.driver.Frame(elapsed)
	if h.panel != nil {
		h.panelStats.Commands = len(frame.Commands)
		frame.Commands = append(frame.Commands, h.panel.Render(frame.Camera, elapsed, h.panelStats)...)
	}
	h.renderer.UpdateResources(h.driver.ConsumeFrameUpdates())
	h.renderer.Render(frame)
}

// panelWantsStats reports whether the dev panel is open and therefore worth
// measuring for. The timings cost a performance.now() call each, which is not
// worth paying for a panel nobody is looking at.
func (h *host) panelWantsStats() bool {
	return h.panel != nil && h.panel.Visible()
}

// samplePanelStats fills in the numbers the panel cannot measure for itself.
// They describe the frame that was just submitted, so the panel shows them
// one frame late — the render time of the frame the panel is being drawn into
// is not known until after it has been drawn.
func (h *host) samplePanelStats(tickStart, tickEnd, inputAtMillis float64) {
	renderEnd := performanceNowMillis()
	h.panelStats.CPUTick = durationFromMillis(tickEnd - tickStart)
	h.panelStats.CPURender = durationFromMillis(renderEnd - tickEnd)
	h.panelStats.CPUSet = true

	if latency, ok := inputLatencySince(inputAtMillis); ok {
		h.panelStats.InputLatency = latency
		h.panelStats.InputLatencySet = true
	}

	used, limit, ok := browserHeapMetrics()
	h.panelStats.HeapInUse, h.panelStats.HeapTotal, h.panelStats.HeapSet = used, limit, ok

	// Go's collector runs in the browser too, and browserHeapMetrics reports
	// the JS heap rather than the Go one, so this is the only view of it.
	if gc, ok := h.gc.Sample(); ok {
		h.panelStats.GCCycles, h.panelStats.GCPause = gc.Cycles, gc.Pause
		h.panelStats.GCAllocBytes = gc.AllocBytes
		h.panelStats.GCHeapLive, h.panelStats.GCHeapGoal = gc.HeapLive, gc.HeapGoal
		h.panelStats.GCSet = true
	}

	if reporter, ok := h.renderer.(gpuTimingReporter); ok {
		h.panelStats.GPUEmitTime, h.panelStats.GPUSimulateTime,
			h.panelStats.GPURenderTime, h.panelStats.GPUTimeSet = reporter.particleGPUTimings()
	}
}

func (h *host) pollGamepad() []input.Event {
	current, present := h.browserGamepadSnapshot()
	h.driver.SetGamepadConnected(present)
	return h.gamepadTracker.Update(present, current)
}

// signalFirstFramePresented sets the arasyFirstFrame <-> shell contract flag
// once, after either a fully opaque splash frame or the first game frame has
// been presented. The shell (internal/wasmserve.ShellHTML) polls this flag to
// reveal the canvas and remove its static poster/progress overlay without
// exposing the splash's transparent fade-in.
func signalFirstFramePresented(presented *bool) {
	if *presented {
		return
	}
	*presented = true
	js.Global().Set("arasyFirstFrame", true)
}
