//go:build js && wasm

package wasm

import (
	"syscall/js"
	"time"
)

const (
	// Five timestamps per frame: [0,1] particle prepare pass (kick + emit)
	// begin/end, [2] simulate pass end, [3,4] render pass begin/end. The
	// simulate duration is [1,2], which includes the prepare→simulate pass
	// boundary — a real GPU cost that belongs to the simulation.
	timingQueryCount = 5
	timingSlotCount  = 4 // readbacks in flight; a frame with no free slot skips sampling
)

// gpuTiming samples GPU-side frame cost with WebGPU timestamp queries. It is
// nil when the adapter does not grant "timestamp-query"; browsers may also
// quantize the reported timestamps (Chromium: 100 µs), which is accurate
// enough for millisecond-scale attribution.
type gpuTiming struct {
	device   js.Value
	querySet js.Value
	resolve  js.Value // QUERY_RESOLVE | COPY_SRC, written by resolveQuerySet
	slots    [timingSlotCount]timingSlot

	prepare  time.Duration
	simulate time.Duration
	render   time.Duration
	valid    bool
}

type timingSlot struct {
	buffer js.Value // MAP_READ | COPY_DST readback target
	busy   bool
	// hasCompute records whether the frame that filled this slot ran timed
	// particle compute passes. Without them, queries [0,2] hold stale values
	// from an older frame and only the render interval [3,4] is meaningful.
	hasCompute bool
}

// newGpuTiming returns nil when the device was created without the
// "timestamp-query" feature, which callers treat as "timing unavailable".
func newGpuTiming(device js.Value) *gpuTiming {
	if !device.Get("features").Call("has", "timestamp-query").Bool() {
		return nil
	}
	t := &gpuTiming{device: device}
	t.querySet = device.Call("createQuerySet", map[string]any{
		"type":  "timestamp",
		"count": timingQueryCount,
	})
	usage := js.Global().Get("GPUBufferUsage")
	t.resolve = device.Call("createBuffer", map[string]any{
		"size":  timingQueryCount * 8,
		"usage": usage.Get("QUERY_RESOLVE").Int() | usage.Get("COPY_SRC").Int(),
	})
	for i := range t.slots {
		t.slots[i].buffer = device.Call("createBuffer", map[string]any{
			"size":  timingQueryCount * 8,
			"usage": usage.Get("MAP_READ").Int() | usage.Get("COPY_DST").Int(),
		})
	}
	return t
}

func (t *gpuTiming) prepareTimestampWrites() map[string]any {
	return map[string]any{
		"querySet":                  t.querySet,
		"beginningOfPassWriteIndex": 0,
		"endOfPassWriteIndex":       1,
	}
}

// simulateTimestampWrites records only the pass end; the simulate duration
// starts at the prepare pass end so the boundary between them is charged to
// the simulation.
func (t *gpuTiming) simulateTimestampWrites() map[string]any {
	return map[string]any{
		"querySet":            t.querySet,
		"endOfPassWriteIndex": 2,
	}
}

func (t *gpuTiming) renderTimestampWrites() map[string]any {
	return map[string]any{
		"querySet":                  t.querySet,
		"beginningOfPassWriteIndex": 3,
		"endOfPassWriteIndex":       4,
	}
}

// renderBeginTimestampWrites/renderEndTimestampWrites split the render
// interval across passes when particle offscreen compositing is active: the
// draw duration then starts at the first offscreen particle pass and ends
// with the main pass, covering the composites and inter-pass gaps.
func (t *gpuTiming) renderBeginTimestampWrites() map[string]any {
	return map[string]any{
		"querySet":                  t.querySet,
		"beginningOfPassWriteIndex": 3,
	}
}

func (t *gpuTiming) renderEndTimestampWrites() map[string]any {
	return map[string]any{
		"querySet":            t.querySet,
		"endOfPassWriteIndex": 4,
	}
}

// encodeResolve records the query resolution and the copy into a free
// readback slot. hasCompute tells the readback whether the particle compute
// queries were written this frame. It returns the slot index, or -1 when
// every readback is still mapped (that frame simply goes unsampled).
func (t *gpuTiming) encodeResolve(encoder js.Value, hasCompute bool) int {
	slot := -1
	for i := range t.slots {
		if !t.slots[i].busy {
			slot = i
			break
		}
	}
	if slot < 0 {
		return -1
	}
	encoder.Call("resolveQuerySet", t.querySet, 0, timingQueryCount, t.resolve, 0)
	encoder.Call("copyBufferToBuffer", t.resolve, 0, t.slots[slot].buffer, 0, timingQueryCount*8)
	t.slots[slot].busy = true
	t.slots[slot].hasCompute = hasCompute
	return slot
}

// readback maps the slot after submit and folds the three u64 nanosecond
// timestamps into compute/render durations. The map completes asynchronously,
// so the published values always lag the frame that produced them by at least
// one frame — fine for a rolling HUD readout.
func (t *gpuTiming) readback(slot int) {
	s := &t.slots[slot]
	var onMap, onErr js.Func
	release := func() {
		onMap.Release()
		onErr.Release()
	}
	onMap = js.FuncOf(func(this js.Value, args []js.Value) any {
		defer release()
		words := js.Global().Get("Uint32Array").New(s.buffer.Call("getMappedRange"))
		t0 := timestampNanos(words, 0)
		t1 := timestampNanos(words, 1)
		t2 := timestampNanos(words, 2)
		t3 := timestampNanos(words, 3)
		t4 := timestampNanos(words, 4)
		s.buffer.Call("unmap")
		s.busy = false
		// Timestamps may be non-monotonic across passes on some drivers;
		// discard the sample rather than publish a negative duration. On
		// frames without timed compute passes the [0,2] queries are stale, so
		// only the render interval is published and the compute costs are
		// zero by definition.
		switch {
		case !s.hasCompute:
			if t4 >= t3 {
				t.prepare = 0
				t.simulate = 0
				t.render = time.Duration(t4 - t3)
				t.valid = true
			}
		case t1 >= t0 && t2 >= t1 && t4 >= t3:
			t.prepare = time.Duration(t1 - t0)
			t.simulate = time.Duration(t2 - t1)
			t.render = time.Duration(t4 - t3)
			t.valid = true
		}
		return nil
	})
	onErr = js.FuncOf(func(this js.Value, args []js.Value) any {
		defer release()
		s.busy = false
		return nil
	})
	mode := js.Global().Get("GPUMapMode").Get("READ")
	s.buffer.Call("mapAsync", mode).Call("then", onMap, onErr)
}

// timestampNanos reassembles the u64 at index i of a Uint32Array view as a
// float64. Nanosecond deltas within a frame lose no precision at this scale.
func timestampNanos(words js.Value, i int) float64 {
	lo := words.Index(i * 2).Float()
	hi := words.Index(i*2 + 1).Float()
	return lo + hi*4294967296
}
