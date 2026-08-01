//go:build windows && amd64

package native

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

	// System probe rows (OS-level; the browser host never sets them).
	MemProcessBytes     uint64
	MemSystemUsedBytes  uint64
	MemSystemTotalBytes uint64
	MemSet              bool

	VRAMProcessBytes uint64
	VRAMBudgetBytes  uint64
	VRAMSet          bool

	GPUProcessPercent float64
	GPUSystemPercent  float64
	GPUSet            bool

	// GPU execution time per pass, from timestamp queries. Unset unless
	// Options.GPUTiming asked for it and the adapter granted the feature.
	GPUEmitTime     time.Duration
	GPUSimulateTime time.Duration
	GPURenderTime   time.Duration
	GPUTimeSet      bool

	// Garbage collection since the previous sample. Counters are read through
	// runtime/metrics, never ReadMemStats: stopping the world to look for
	// pauses would be self-defeating.
	GCCycles     uint64
	GCPause      time.Duration
	GCAllocBytes uint64
	GCHeapLive   uint64
	GCHeapGoal   uint64
	GCSet        bool
	GCFresh      bool

	// PresentWait is the presentation-queue block of the last frame — the
	// vblank wait under vsync, reported apart from CPU render time.
	PresentWait time.Duration
	PresentSet  bool
}
