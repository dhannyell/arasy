//go:build (windows || linux || darwin) && (amd64 || arm64) && arr_dev

package native

import (
	"time"

	metrics "runtime/metrics"

	"github.com/dhannyell/arasy/input"
	"github.com/dhannyell/arasy/internal/devpanel"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
	"github.com/dhannyell/arasy/runtime"
)

// panelOverlay adapts the dev panel to the host's devOverlay seam.
type panelOverlay struct {
	panel *devpanel.Panel
}

func devToolsWantGPUTiming(enabled bool) bool { return enabled }

func (o *panelOverlay) HandleInput(events []input.Event, viewport math2d.Rect) []input.Event {
	return o.panel.HandleInput(events, viewport)
}

func (o *panelOverlay) Render(cam render.Camera, elapsed time.Duration, stats overlayStats) []render.Command {
	return o.panel.Render(cam, elapsed, devpanel.Stats{
		Commands:        stats.Commands,
		CPUTick:         stats.CPUTick,
		CPURender:       stats.CPURender,
		CPUSet:          stats.CPUSet,
		InputLatency:    stats.InputLatency,
		InputLatencySet: stats.InputLatencySet,
		HeapInUse:       stats.HeapInUse,
		HeapTotal:       stats.HeapTotal,
		HeapSet:         stats.HeapSet,

		MemProcessBytes:     stats.MemProcessBytes,
		MemSystemUsedBytes:  stats.MemSystemUsedBytes,
		MemSystemTotalBytes: stats.MemSystemTotalBytes,
		MemSet:              stats.MemSet,
		VRAMProcessBytes:    stats.VRAMProcessBytes,
		VRAMBudgetBytes:     stats.VRAMBudgetBytes,
		VRAMSet:             stats.VRAMSet,
		GPUProcessPercent:   stats.GPUProcessPercent,
		GPUSystemPercent:    stats.GPUSystemPercent,
		GPUSet:              stats.GPUSet,
		GPUEmitTime:         stats.GPUEmitTime,
		GPUSimulateTime:     stats.GPUSimulateTime,
		GPURenderTime:       stats.GPURenderTime,
		GPUTimeSet:          stats.GPUTimeSet,
		GCCycles:            stats.GCCycles,
		GCPause:             stats.GCPause,
		GCAllocBytes:        stats.GCAllocBytes,
		GCHeapLive:          stats.GCHeapLive,
		GCHeapGoal:          stats.GCHeapGoal,
		GCSet:               stats.GCSet,
		GCFresh:             stats.GCFresh,
		PresentWait:         stats.PresentWait,
		PresentSet:          stats.PresentSet,
	})
}

func (o *panelOverlay) Toggle()       { o.panel.Toggle() }
func (o *panelOverlay) Visible() bool { return o.panel.Visible() }

// mountDevPanel builds the tooling overlay when the app asked for it. The
// panel itself is shared with the browser host; what is native about it is
// the directory it writes recordings into.
func (h *host) mountDevPanel(opts Options) {
	var newGame func() runtime.Game
	if opts.RecordReplay {
		if opts.GameFactory == nil {
			opts.Log.Error("RecordReplay enabled without GameFactory; recording controls disabled")
		} else {
			newGame = opts.GameFactory
		}
	}
	if !opts.DevTools && newGame == nil {
		return
	}

	panelOpts := devpanel.Options{
		DevMode: opts.DevTools,
		NewGame: newGame,
		Log:     opts.Log,
		Adopt:   func(driver *runtime.Driver) { h.driver = driver },
	}
	if newGame != nil {
		panelOpts.Recording = newRecordingPort()
	}
	h.panel = &panelOverlay{panel: devpanel.New(h.driver, panelOpts)}

	// The panel splits CPU work from the vblank wait; the wait is only
	// measured when CPU timing is on, so turn it on for the panel too.
	h.renderer.SetCPUTiming(true)

	// The panel reports heap use, which is what a collector pause looks like
	// before it becomes a frame spike. Sampling it needs the same probes the
	// metrics reporter uses.
	if h.heapSamples == nil {
		h.heapSamples = []metrics.Sample{
			{Name: "/memory/classes/heap/objects:bytes"},
			{Name: "/memory/classes/total:bytes"},
		}
	}
}
