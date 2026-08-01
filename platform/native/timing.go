//go:build (windows || linux || darwin) && (amd64 || arm64)

package native

import (
	"encoding/binary"
	"time"

	"github.com/oliverbestmann/webgpu/wgpu"
)

const (
	// Five timestamps per frame: [0,1] particle emit pass begin/end, [2]
	// simulate pass end, [3,4] render begin/end. The simulate duration is
	// [1,2], which includes the emit→simulate pass boundary — a real GPU cost
	// that belongs to the simulation. Mirrors the WASM host's layout so the two
	// backends' HUD rows mean the same thing.
	timingQueryCount = 5
	// timingRenderBegin is where the render pair starts in the set. A frame
	// without particle work resolves from here instead of from zero.
	timingRenderBegin = 3
	timingSlotCount   = 4 // readbacks in flight; a frame with no free slot skips sampling
	timingSampleSize  = timingQueryCount * 8
	// timingInterval is how many frames pass between samples. Timestamp writes
	// are not free: they punctuate the pass boundaries the driver would
	// otherwise be free to overlap, and on a frame that costs half a
	// millisecond that punctuation is a measurable share of the frame. Sampling
	// every frame buys a resolution the HUD cannot show — it redraws a handful
	// of times a second — at a cost the frame rate can feel.
	timingInterval = 8
)

// gpuTiming samples GPU-side frame cost with timestamp queries. It is nil when
// the device was created without FeatureNameTimestampQuery, which callers treat
// as "timing unavailable".
//
// Unlike the browser, wgpu-native resolves timestamps in driver ticks rather
// than nanoseconds, so every delta is scaled by the queue's timestamp period.
type gpuTiming struct {
	device *wgpu.Device
	queue  *wgpu.Queue

	querySet *wgpu.QuerySet
	resolve  *wgpu.Buffer // QueryResolve | CopySrc, written by ResolveQuerySet
	slots    [timingSlotCount]timingSlot

	prepare  time.Duration
	simulate time.Duration
	render   time.Duration
	valid    bool

	frame int // counts frames to space samples timingInterval apart
}

// sampleFrame reports whether this frame should carry timestamp writes. Frames
// in between are encoded exactly as they would be with timing off, which is the
// whole point: the cost is paid once every timingInterval frames instead of
// every frame.
func (t *gpuTiming) sampleFrame() bool {
	if t == nil {
		return false
	}
	sample := t.frame == 0
	t.frame = (t.frame + 1) % timingInterval
	return sample
}

type timingSlot struct {
	buffer *wgpu.Buffer // MapRead | CopyDst readback target
	busy   bool
	// hasCompute records whether the frame that filled this slot ran the
	// particle compute passes, which decides how many queries were resolved
	// into the buffer and therefore how to read it back.
	hasCompute bool
	// mapped and status are written by the map callback and consumed by drain.
	mapped bool
	status wgpu.MapAsyncStatus
}

// newGPUTiming returns nil when the device lacks the timestamp-query feature,
// and an error only when the feature is present but its resources cannot be
// created — a case the caller reports rather than silently downgrades.
func newGPUTiming(d *device) (*gpuTiming, error) {
	if d == nil || !d.timestampQuery {
		return nil, nil
	}
	t := &gpuTiming{device: d.device, queue: d.queue}
	var err error
	if t.querySet, err = d.device.TryCreateQuerySet(&wgpu.QuerySetDescriptor{
		Label: "Arasy timestamps",
		Type:  wgpu.QueryTypeTimestamp,
		Count: timingQueryCount,
	}); err != nil {
		return nil, err
	}
	if t.resolve, err = d.device.TryCreateBuffer(&wgpu.BufferDescriptor{
		Label: "Arasy timestamp resolve",
		Size:  timingSampleSize,
		Usage: wgpu.BufferUsageQueryResolve | wgpu.BufferUsageCopySrc,
	}); err != nil {
		t.release()
		return nil, err
	}
	for i := range t.slots {
		if t.slots[i].buffer, err = d.device.TryCreateBuffer(&wgpu.BufferDescriptor{
			Label: "Arasy timestamp readback",
			Size:  timingSampleSize,
			Usage: wgpu.BufferUsageMapRead | wgpu.BufferUsageCopyDst,
		}); err != nil {
			t.release()
			return nil, err
		}
	}
	return t, nil
}

// prepareWrites and simulateWrites time the two particle compute passes. The
// simulate pass records only its end, so the boundary before it is charged to
// the simulation rather than disappearing between the two.
func (t *gpuTiming) prepareWrites() *wgpu.PassTimestampWrites {
	if t == nil {
		return nil
	}
	return &wgpu.PassTimestampWrites{QuerySet: t.querySet, BeginningOfPassWriteIndex: 0, EndOfPassWriteIndex: 1}
}

func (t *gpuTiming) simulateWrites() *wgpu.PassTimestampWrites {
	if t == nil {
		return nil
	}
	return &wgpu.PassTimestampWrites{
		QuerySet:                  t.querySet,
		BeginningOfPassWriteIndex: wgpu.QuerySetIndexUndefined,
		EndOfPassWriteIndex:       2,
	}
}

// renderWrites times the main pass alone. renderBeginWrites/renderEndWrites
// split the same interval across passes when particle offscreen compositing is
// active: the draw duration then starts at the first offscreen particle pass
// and ends with the main pass, covering the composites and inter-pass gaps.
func (t *gpuTiming) renderWrites() *wgpu.PassTimestampWrites {
	if t == nil {
		return nil
	}
	return &wgpu.PassTimestampWrites{QuerySet: t.querySet, BeginningOfPassWriteIndex: 3, EndOfPassWriteIndex: 4}
}

func (t *gpuTiming) renderBeginWrites() *wgpu.PassTimestampWrites {
	if t == nil {
		return nil
	}
	return &wgpu.PassTimestampWrites{
		QuerySet:                  t.querySet,
		BeginningOfPassWriteIndex: 3,
		EndOfPassWriteIndex:       wgpu.QuerySetIndexUndefined,
	}
}

func (t *gpuTiming) renderEndWrites() *wgpu.PassTimestampWrites {
	if t == nil {
		return nil
	}
	return &wgpu.PassTimestampWrites{
		QuerySet:                  t.querySet,
		BeginningOfPassWriteIndex: wgpu.QuerySetIndexUndefined,
		EndOfPassWriteIndex:       4,
	}
}

// encodeResolve records the query resolution and the copy into a free readback
// slot. It returns the slot index, or -1 when every readback is still mapped,
// in which case the frame simply goes unsampled.
//
// hasCompute says whether the particle compute passes ran this frame. When
// they did not, queries 0 to 2 were never written, and resolving a query that
// was never written is undefined behaviour — on Vulkan it loses the device
// outright. So a frame without particle work resolves only the render pair,
// which lands at the front of the buffer and is read back as such.
func (t *gpuTiming) encodeResolve(encoder *wgpu.CommandEncoder, hasCompute bool) int {
	if t == nil {
		return -1
	}
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
	first, count := uint32(timingRenderBegin), uint32(2)
	if hasCompute {
		first, count = 0, timingQueryCount
	}
	if err := encoder.TryResolveQuerySet(t.querySet, first, count, t.resolve, 0); err != nil {
		return -1
	}
	if err := encoder.TryCopyBufferToBuffer(t.resolve, 0, t.slots[slot].buffer, 0, uint64(count)*8); err != nil {
		return -1
	}
	t.slots[slot].busy = true
	t.slots[slot].hasCompute = hasCompute
	return slot
}

// readback maps the slot after submit and folds the five timestamps into
// prepare/simulate/render durations. The map completes asynchronously and is
// pumped by poll, so the published values always lag the frame that produced
// them — fine for a rolling HUD readout, and the reason this never blocks the
// frame loop waiting on the GPU.
// The callback itself only records what happened. It runs on the cgo side of
// Poll, where calling back into the binding would trip the runtime's
// nocallback guard, so reading the mapped bytes is left to drain.
func (t *gpuTiming) readback(slot int) {
	if t == nil || slot < 0 {
		return
	}
	s := &t.slots[slot]
	err := s.buffer.TryMapAsync(wgpu.MapModeRead, 0, timingSampleSize, func(status wgpu.MapAsyncStatus) {
		s.status = status
		s.mapped = true
	})
	if err != nil {
		s.busy = false
	}
}

// drain consumes every slot whose map callback has fired since the last frame,
// back on the Go side where calling into the binding is safe again.
func (t *gpuTiming) drain() {
	if t == nil {
		return
	}
	for i := range t.slots {
		s := &t.slots[i]
		if !s.mapped {
			continue
		}
		s.mapped = false
		if s.status == wgpu.MapAsyncStatusSuccess {
			size := 2 * 8
			if s.hasCompute {
				size = timingSampleSize
			}
			if data := s.buffer.GetMappedRange(0, uint(size)); len(data) >= size {
				var ticks [timingQueryCount]uint64
				for j := range size / 8 {
					ticks[j] = binary.LittleEndian.Uint64(data[j*8:])
				}
				t.publish(ticks, s.hasCompute)
			}
			s.buffer.Unmap()
		}
		s.busy = false
	}
}

// publish converts a resolved sample into durations, scaling driver ticks by
// the queue's timestamp period. Timestamps may be non-monotonic across passes
// on some drivers, so a sample that would yield a negative duration is
// discarded rather than shown.
//
// Without compute work only the render pair was resolved, so it sits at the
// front of ticks and the compute stages report zero — which is what they cost.
func (t *gpuTiming) publish(ticks [timingQueryCount]uint64, hasCompute bool) {
	if !hasCompute {
		if ticks[1] >= ticks[0] {
			t.prepare, t.simulate = 0, 0
			t.render = t.scale(ticks[0], ticks[1])
			t.valid = true
		}
		return
	}
	if ticks[1] < ticks[0] || ticks[2] < ticks[1] || ticks[4] < ticks[3] {
		return
	}
	t.prepare = t.scale(ticks[0], ticks[1])
	t.simulate = t.scale(ticks[1], ticks[2])
	t.render = t.scale(ticks[3], ticks[4])
	t.valid = true
}

// scale turns a tick interval into a duration. wgpu-native resolves timestamps
// in driver ticks rather than nanoseconds, so the queue's period is what makes
// the number comparable across machines.
func (t *gpuTiming) scale(from, to uint64) time.Duration {
	period := float64(t.queue.GetTimestampPeriod())
	if period <= 0 {
		period = 1
	}
	return time.Duration(float64(to-from) * period)
}

// poll pumps pending map callbacks without blocking. Native mapping only
// progresses while the device is polled, and the frame loop must not stall on
// the GPU, so this is called once per frame and picks up whatever is ready.
func (t *gpuTiming) poll() {
	if t == nil {
		return
	}
	t.device.Poll(false, nil)
}

// timings reports the most recent resolved sample. ok is false until the first
// readback completes.
func (t *gpuTiming) timings() (prepare, simulate, render time.Duration, ok bool) {
	if t == nil || !t.valid {
		return 0, 0, 0, false
	}
	return t.prepare, t.simulate, t.render, true
}

func (t *gpuTiming) release() {
	if t == nil {
		return
	}
	for i := range t.slots {
		if t.slots[i].buffer != nil {
			t.slots[i].buffer.Release()
			t.slots[i].buffer = nil
		}
	}
	if t.resolve != nil {
		t.resolve.Release()
		t.resolve = nil
	}
	if t.querySet != nil {
		t.querySet.Release()
		t.querySet = nil
	}
}
