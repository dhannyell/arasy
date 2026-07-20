//go:build js && wasm

package wasm

import (
	"sort"
	"syscall/js"
	"time"
)

const (
	frameMetricWindow         = 240
	frameMetricReportInterval = 250 * time.Millisecond
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
	// share of CPURenderTime: tessellation into vertex streams, vertex
	// upload (JS heap copy + writeBuffer) and command encoding + submit.
	// WebGPU only; the difference CPURenderTime - (sum of the three) is the
	// game's command building and dispatch.
	CPUTessTime     time.Duration
	CPUUploadTime   time.Duration
	CPUEncodeTime   time.Duration
	CPUBreakdownSet bool
}

type frameMetricTracker struct {
	samples       [frameMetricWindow]time.Duration
	scratch       [frameMetricWindow]time.Duration
	next          int
	count         int
	reportElapsed time.Duration
	lastInput     time.Duration
	hasInput      bool
}

func (t *frameMetricTracker) Observe(
	frameTime, inputToSubmit time.Duration,
	inputToSubmitSet bool,
) (FrameMetrics, bool) {
	if frameTime <= 0 {
		return FrameMetrics{}, false
	}
	t.samples[t.next] = frameTime
	t.next = (t.next + 1) % len(t.samples)
	if t.count < len(t.samples) {
		t.count++
	}
	if inputToSubmitSet {
		t.lastInput = inputToSubmit
		t.hasInput = true
	}
	t.reportElapsed += frameTime
	if t.reportElapsed < frameMetricReportInterval {
		return FrameMetrics{}, false
	}
	t.reportElapsed = 0

	var total time.Duration
	for i := 0; i < t.count; i++ {
		total += t.samples[i]
		t.scratch[i] = t.samples[i]
	}
	sort.Slice(t.scratch[:t.count], func(i, j int) bool { return t.scratch[i] < t.scratch[j] })
	mean := total / time.Duration(t.count)
	rank := (99*t.count + 99) / 100 // ceil(0.99*n), one-based.
	return FrameMetrics{
		FPS:              float64(time.Second) / float64(mean),
		FrameTime:        frameTime,
		MeanFrameTime:    mean,
		P99FrameTime:     t.scratch[rank-1],
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
