//go:build js && wasm

package wasm

import (
	"time"

	"github.com/dhannyell/arasy/input"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
)

// devOverlay is the seam between the host loop and the development tooling
// overlay. Only builds tagged arr_dev back it with the dev panel
// (devtools_dev.go); every other build leaves it nil (devtools_release.go),
// so a shipped game cannot open the panel — its code is not in the binary.
type devOverlay interface {
	HandleInput(events []input.Event, viewport math2d.Rect) []input.Event
	Render(cam render.Camera, elapsed time.Duration, stats overlayStats) []render.Command
	Toggle()
	Visible() bool
}

// overlayStats mirrors the panel's Stats struct field for field. The host
// cannot name devpanel.Stats without importing the panel package, which is
// exactly what the release build must not do.
type overlayStats struct {
	Commands int

	CPUTick   time.Duration
	CPURender time.Duration
	CPUSet    bool

	InputLatency    time.Duration
	InputLatencySet bool

	HeapInUse uint64
	HeapTotal uint64
	HeapSet   bool

	// GPU execution time per pass, from timestamp queries. Unset on adapters
	// that do not grant "timestamp-query", and on the Canvas2D fallback.
	GPUEmitTime     time.Duration
	GPUSimulateTime time.Duration
	GPURenderTime   time.Duration
	GPUTimeSet      bool

	// Garbage collection since the previous frame. Cheap counters read through
	// runtime/metrics, never ReadMemStats: stopping the world to look for
	// pauses would be self-defeating.
	GCCycles     uint64
	GCPause      time.Duration
	GCAllocBytes uint64
	GCHeapLive   uint64
	GCHeapGoal   uint64
	GCSet        bool
}
