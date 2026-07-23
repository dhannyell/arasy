//go:build js && wasm

package wasm

import (
	"syscall/js"
	"time"

	"github.com/dhannyell/frameforge/runtime"
)

// FrameMetrics describes browser frame pacing over the most recent rolling
// window. FrameTime includes the requestAnimationFrame interval, so it captures
// CPU, GPU and display pacing together. InputToSubmit is present only after a
// browser input event and ends once rendering for that event has been submitted.
type FrameMetrics struct {
	FPS              float64
	FrameTime        time.Duration
	MeanFrameTime    time.Duration
	P99FrameTime     time.Duration
	InputToSubmit    time.Duration
	InputToSubmitSet bool
	// JSHeap reports Chromium's non-standard performance.memory counters when
	// the browser exposes them. It is browser-managed heap, not total process
	// RAM or the WebGPU driver's allocation.
	JSHeapUsed  uint64
	JSHeapLimit uint64
	JSHeapSet   bool
	// GPUBufferBytes is the renderer's known particle storage allocation. The
	// WebGPU API does not expose adapter-wide VRAM use or utilization percent.
	GPUBufferBytes uint64
	GPUBufferSet   bool
	// GPUPrepareTime, GPUSimulateTime and GPURenderTime are the most recent
	// timestamp-query sample: the particle prepare pass (kick + emit), the
	// simulate pass and the main render pass. Set only on WebGPU adapters
	// that grant "timestamp-query", and the sample lags the frame that
	// produced it because the readback is asynchronous.
	GPUPrepareTime  time.Duration
	GPUSimulateTime time.Duration
	GPURenderTime   time.Duration
	GPUTimingSet    bool
	// CPUTickTime and CPURenderTime are the Go-side wall times of the frame
	// that produced this report: the simulation tick (RunTick + audio
	// consume) and the render traversal (RenderFrame: the game's command
	// building plus the renderer). Their sum is the loop's share of
	// FrameTime; the remainder is browser scheduling and presentation.
	CPUTickTime   time.Duration
	CPURenderTime time.Duration
	CPUTimingSet  bool
	// CPUTessTime, CPUUploadTime and CPUEncodeTime split the renderer's
	// share of CPURenderTime: geometry/storage-record construction, GPU-buffer
	// upload (JS heap copy + writeBuffer) and command encoding + submit.
	// WebGPU only; the difference CPURenderTime - (sum of the three) is the
	// game's command building and dispatch.
	CPUTessTime     time.Duration
	CPUUploadTime   time.Duration
	CPUEncodeTime   time.Duration
	CPUBreakdownSet bool
	// Retained holds retained sprite/text batch accounting: GPU-resident
	// bytes across every registered batch, content bytes actually uploaded
	// this frame, packed vs. visible (post-culling) instance/label counts,
	// and the number of retained draw calls issued. WebGPU only.
	Retained    RetainedBatchMetrics
	RetainedSet bool
}

// RetainedBatchMetrics summarizes retained sprite/text batch state for one
// reported frame. ContentBytes is zero at steady state: unchanged content,
// projection and atlas identity mean no batch repacks and nothing uploads.
type RetainedBatchMetrics struct {
	GPUBytes                     uint64
	ContentBytes                 uint64
	SpritesTotal, SpritesVisible int
	LabelsTotal, LabelsVisible   int
	Draws                        int
}

type frameMetricTracker struct {
	stats     runtime.FrameStatsTracker
	lastInput time.Duration
	hasInput  bool
}

func (t *frameMetricTracker) Observe(
	frameTime, inputToSubmit time.Duration,
	inputToSubmitSet bool,
) (FrameMetrics, bool) {
	if frameTime <= 0 {
		return FrameMetrics{}, false
	}
	if inputToSubmitSet {
		t.lastInput = inputToSubmit
		t.hasInput = true
	}
	stats, ready := t.stats.Observe(frameTime)
	if !ready {
		return FrameMetrics{}, false
	}
	return FrameMetrics{
		FPS:              stats.FPS,
		FrameTime:        stats.FrameTime,
		MeanFrameTime:    stats.MeanFrameTime,
		P99FrameTime:     stats.P99FrameTime,
		InputToSubmit:    t.lastInput,
		InputToSubmitSet: t.hasInput,
	}, true
}

func performanceNowMillis() float64 {
	return js.Global().Get("performance").Call("now").Float()
}

func durationFromMillis(ms float64) time.Duration {
	if ms < 0 {
		return 0
	}
	return time.Duration(ms * float64(time.Millisecond))
}

func inputLatencySince(inputAtMillis float64) (time.Duration, bool) {
	if inputAtMillis < 0 {
		return 0, false
	}
	latencyMillis := performanceNowMillis() - inputAtMillis
	if latencyMillis < 0 {
		return 0, false
	}
	return time.Duration(latencyMillis * float64(time.Millisecond)), true
}

func browserHeapMetrics() (used, limit uint64, ok bool) {
	memory := js.Global().Get("performance").Get("memory")
	if memory.IsUndefined() || memory.IsNull() {
		return 0, 0, false
	}
	usedValue := memory.Get("usedJSHeapSize")
	limitValue := memory.Get("jsHeapSizeLimit")
	if usedValue.IsUndefined() || usedValue.IsNull() || limitValue.IsUndefined() || limitValue.IsNull() {
		return 0, 0, false
	}
	return uint64(usedValue.Float()), uint64(limitValue.Float()), true
}

type gpuBufferReporter interface {
	particleGPUBufferBytes() uint64
}

type gpuTimingReporter interface {
	particleGPUTimings() (prepare, simulate, render time.Duration, ok bool)
}

type cpuTimingReporter interface {
	renderCPUTimings() (tessTime, upload, encode time.Duration, ok bool)
}

type retainedBatchReporter interface {
	retainedBatchMetrics() (RetainedBatchMetrics, bool)
}
