//go:build arr_dev

package devpanel

import (
	"testing"
	"time"

	"github.com/dhannyell/arasy/render"
)

// The panel measures pacing itself so the tab works on a host that was never
// asked for metrics — which is every host, by default.
func TestPerfTracksFramePacingWithoutHostHelp(t *testing.T) {
	p := openPanel(t, Options{}, tabPerf)

	// The tracker reports on a ~250ms cadence, so feed it a second of frames.
	for range 60 {
		p.Render(frameCamera(), 16*time.Millisecond, Stats{})
	}

	if p.frameStats.FPS <= 0 {
		t.Fatalf("fps = %v, want a positive rate after a second of frames", p.frameStats.FPS)
	}
	if p.frameStats.P99FrameTime <= 0 {
		t.Fatal("p99 never reported; an average alone hides the spikes this tab exists for")
	}
}

// A closed panel still samples: a history that began when the panel opened
// would be blank exactly when somebody opens it to find out what happened.
func TestPerfKeepsSamplingWhileClosed(t *testing.T) {
	p := newTestPanel(t, Options{})

	for range 10 {
		p.Render(frameCamera(), 16*time.Millisecond, Stats{})
	}

	if got := p.history.count(); got != 10 {
		t.Fatalf("closed panel recorded %d frames, want 10", got)
	}
}

// The ring keeps the most recent frames and hands them back oldest first, so
// the graph scrolls in one direction instead of jumping at the wrap.
func TestFrameHistoryWrapsInOrder(t *testing.T) {
	var history frameHistory
	for i := range frameHistoryLen + 5 {
		history.push(time.Duration(i+1)*time.Millisecond, false)
	}

	if got := history.count(); got != frameHistoryLen {
		t.Fatalf("history holds %d samples, want %d", got, frameHistoryLen)
	}

	var seen []time.Duration
	history.each(func(_ int, frame time.Duration, _ bool) { seen = append(seen, frame) })

	if len(seen) != frameHistoryLen {
		t.Fatalf("walked %d samples, want %d", len(seen), frameHistoryLen)
	}
	// The five oldest were overwritten, so the walk starts at the sixth push.
	if want := 6 * time.Millisecond; seen[0] != want {
		t.Fatalf("oldest sample = %v, want %v", seen[0], want)
	}
	for i := 1; i < len(seen); i++ {
		if seen[i] <= seen[i-1] {
			t.Fatalf("sample %d (%v) is not newer than %d (%v)", i, seen[i], i-1, seen[i-1])
		}
	}
}

// A zero or negative delta is not a frame; letting one in would put a bar of
// nothing in the graph and drag the average down.
func TestFrameHistoryIgnoresEmptyFrames(t *testing.T) {
	var history frameHistory
	history.push(0, false)
	history.push(-time.Millisecond, false)

	if got := history.count(); got != 0 {
		t.Fatalf("history recorded %d non-frames, want none", got)
	}
}

// The budget comes from the game's configured frame rate: a game running at
// 30 has a different idea of a late frame than one running at 60.
func TestFrameBudgetFollowsConfiguredFPS(t *testing.T) {
	p := newTestPanel(t, Options{})
	if got, want := p.frameBudget(), time.Second/60; got != want {
		t.Fatalf("budget = %v, want %v", got, want)
	}
}

// Host extras are optional: a host that measures none of them still gets a
// tab, it just shows fewer lines.
func TestPerfDrawsWithAndWithoutHostExtras(t *testing.T) {
	p := openPanel(t, Options{}, tabPerf)

	bare := p.Render(frameCamera(), 16*time.Millisecond, Stats{Commands: 10})
	if len(bare) == 0 {
		t.Fatal("perf tab drew nothing without host stats")
	}

	full := p.Render(frameCamera(), 16*time.Millisecond, Stats{
		Commands:        10,
		CPUTick:         time.Millisecond,
		CPURender:       2 * time.Millisecond,
		CPUSet:          true,
		InputLatency:    8 * time.Millisecond,
		InputLatencySet: true,
		HeapInUse:       4 << 20,
		HeapTotal:       16 << 20,
		HeapSet:         true,
	})
	if len(full) <= len(bare) {
		t.Fatalf("perf tab drew %d commands with host stats and %d without, want more",
			len(full), len(bare))
	}
}

// Frames over budget are marked, because "some frames are late" is the whole
// question the tab answers and an average will not show it.
func TestPerfSystemRowsFollowTheirSetFlags(t *testing.T) {
	p := openPanel(t, Options{}, tabPerf)

	bare := p.Render(frameCamera(), 16*time.Millisecond, Stats{Commands: 10})
	if containsText(bare, "gpu 12%   sys 47%") {
		t.Fatal("system rows drew without their Set flags")
	}

	full := p.Render(frameCamera(), 16*time.Millisecond, Stats{
		Commands:            10,
		MemProcessBytes:     142 << 20,
		MemSystemUsedBytes:  9 << 30,
		MemSystemTotalBytes: 16 << 30,
		MemSet:              true,
		VRAMProcessBytes:    320 << 20,
		VRAMBudgetBytes:     12 << 30,
		VRAMSet:             true,
		GPUProcessPercent:   12,
		GPUSystemPercent:    47,
		GPUSet:              true,
	})
	requireText(t, full, "mem 142.0MB   sys 9.0GB / 16.0GB")
	requireText(t, full, "vram 320.0MB / 12.0GB")
	requireText(t, full, "gpu 12%   sys 47%")
}

// GPU pass times are their own row and their own flag: a host that reports
// utilization may still have no timestamp queries, and one that has timestamps
// may run where utilization cannot be probed.
func TestPerfGPUTimeRowFollowsItsOwnFlag(t *testing.T) {
	p := openPanel(t, Options{}, tabPerf)

	utilizationOnly := p.Render(frameCamera(), 16*time.Millisecond, Stats{
		Commands:          10,
		GPUProcessPercent: 12,
		GPUSystemPercent:  47,
		GPUSet:            true,
	})
	requireText(t, utilizationOnly, "gpu 12%   sys 47%")
	if containsText(utilizationOnly, "gpu emit") {
		t.Fatal("GPU time row drew without GPUTimeSet")
	}

	// Sub-millisecond passes must not all collapse to "0.0ms": at that
	// resolution the row would report nothing on a healthy frame.
	timed := p.Render(frameCamera(), 16*time.Millisecond, Stats{
		Commands:        10,
		GPUEmitTime:     3 * time.Microsecond,
		GPUSimulateTime: 210 * time.Microsecond,
		GPURenderTime:   4200 * time.Microsecond,
		GPUTimeSet:      true,
	})
	requireText(t, timed, "gpu emit 3us   sim 210us   draw 4.2ms")
}

// The collector's bar has to be told apart from a late bar. "It stutters
// sometimes" is the symptom this tab exists to name, and a collection is the
// answer often enough that it deserves its own colour rather than sharing the
// one that only says "late".
func TestPerfMarksCollectedFrames(t *testing.T) {
	p := openPanel(t, Options{}, tabPerf)

	// Frames well inside the budget, so nothing can be marked as late.
	for range 20 {
		p.Render(frameCamera(), 4*time.Millisecond, Stats{GCSet: true})
	}
	clean := countFill(p.Render(frameCamera(), 4*time.Millisecond, Stats{GCSet: true}), colorCollected)
	if clean != 0 {
		t.Fatalf("%d bars marked as collected while no cycle ran", clean)
	}

	p.Render(frameCamera(), 4*time.Millisecond, Stats{GCSet: true, GCFresh: true, GCCycles: 1})
	marked := countFill(p.Render(frameCamera(), 4*time.Millisecond, Stats{GCSet: true}), colorCollected)
	if marked != 1 {
		t.Fatalf("%d bars marked as collected, want exactly the one frame that collected", marked)
	}
}

// A collection outranks "late": a late frame poses the question and the
// collection answers it, so the bar must not stay orange and hide the cause.
func TestPerfCollectionOutranksLateness(t *testing.T) {
	p := openPanel(t, Options{}, tabPerf)

	p.Render(frameCamera(), 40*time.Millisecond, Stats{GCSet: true, GCFresh: true, GCCycles: 1})
	cmds := p.Render(frameCamera(), 4*time.Millisecond, Stats{GCSet: true})
	if late := countFill(cmds, colorWarning); late != 0 {
		t.Fatalf("%d bars kept the late colour on a frame that collected", late)
	}
	if collected := countFill(cmds, colorCollected); collected != 1 {
		t.Fatalf("%d bars marked as collected, want 1", collected)
	}
}

// The row leads with the allocation rate because that is the part the game's
// code can change; the cycle count only joins it when a cycle actually ran.
func TestPerfGCRowFollowsItsFlag(t *testing.T) {
	p := openPanel(t, Options{}, tabPerf)

	if containsText(p.Render(frameCamera(), 16*time.Millisecond, Stats{}), "alloc ") {
		t.Fatal("GC row drew without GCSet")
	}

	quiet := p.Render(frameCamera(), 16*time.Millisecond, Stats{
		GCAllocBytes: 2 << 20,
		GCHeapLive:   8 << 20,
		GCHeapGoal:   16 << 20,
		GCSet:        true,
	})
	requireText(t, quiet, "alloc 2.0MB   heap 8.0MB / 16.0MB")
	if containsText(quiet, "   gc ") {
		t.Fatal("cycle count drew on a frame where no cycle ran")
	}

	collected := p.Render(frameCamera(), 16*time.Millisecond, Stats{
		GCAllocBytes: 2 << 20,
		GCHeapLive:   8 << 20,
		GCHeapGoal:   16 << 20,
		GCCycles:     1,
		GCPause:      120 * time.Microsecond,
		GCSet:        true,
	})
	requireText(t, collected, "alloc 2.0MB   heap 8.0MB / 16.0MB   gc 1 120us")
}

func TestPerfSplitsPresentWaitFromRenderWork(t *testing.T) {
	p := openPanel(t, Options{}, tabPerf)

	cmds := p.Render(frameCamera(), 16*time.Millisecond, Stats{
		Commands:    17,
		CPUTick:     300 * time.Microsecond,
		CPURender:   16400 * time.Microsecond,
		CPUSet:      true,
		PresentWait: 16100 * time.Microsecond,
		PresentSet:  true,
	})
	requireText(t, cmds, "cpu tick 0.3ms   render 16.4ms   present 16.1ms")

	// Without the probe the line keeps the old shape, render unadjusted.
	bare := p.Render(frameCamera(), 16*time.Millisecond, Stats{
		Commands:  17,
		CPUTick:   300 * time.Microsecond,
		CPURender: 16400 * time.Microsecond,
		CPUSet:    true,
	})
	requireText(t, bare, "cpu tick 0.3ms   render 16.4ms")
}

func TestPerfMarksFramesOverBudget(t *testing.T) {
	p := openPanel(t, Options{}, tabPerf)

	for range 30 {
		p.Render(frameCamera(), 8*time.Millisecond, Stats{})
	}
	onTime := countFill(p.Render(frameCamera(), 8*time.Millisecond, Stats{}), colorWarning)
	if onTime != 0 {
		t.Fatalf("%d bars marked late while every frame was inside the budget", onTime)
	}

	p.Render(frameCamera(), 40*time.Millisecond, Stats{})
	cmds := p.Render(frameCamera(), 8*time.Millisecond, Stats{})
	late := countFill(cmds, colorWarning)
	if late == 0 {
		t.Fatal("a 40ms frame was not marked as over budget")
	}
	if batches := countTriangleBatches(cmds, colorWarning); batches != 1 {
		t.Fatalf("late bars used %d draw batches, want 1", batches)
	}
}

func countTriangleBatches(commands []render.Command, color render.Color) int {
	count := 0
	for _, command := range commands {
		if triangles, ok := command.(render.DrawTriangles); ok && triangles.Color == color {
			count++
		}
	}
	return count
}

func countFill(commands []render.Command, fill render.Color) int {
	count := 0
	for _, command := range commands {
		switch command := command.(type) {
		case render.DrawRect:
			if command.Fill == fill {
				count++
			}
		case render.DrawTriangles:
			if command.Color == fill {
				count += len(command.Indices) / 6
			}
		}
	}
	return count
}
