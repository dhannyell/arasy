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
		history.push(time.Duration(i+1) * time.Millisecond)
	}

	if got := history.count(); got != frameHistoryLen {
		t.Fatalf("history holds %d samples, want %d", got, frameHistoryLen)
	}

	var seen []time.Duration
	history.each(func(_ int, frame time.Duration) { seen = append(seen, frame) })

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
	history.push(0)
	history.push(-time.Millisecond)

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
func TestPerfMarksFramesOverBudget(t *testing.T) {
	p := openPanel(t, Options{}, tabPerf)

	for range 30 {
		p.Render(frameCamera(), 8*time.Millisecond, Stats{})
	}
	onTime := countFill(p.Render(frameCamera(), 8*time.Millisecond, Stats{}), perfOverBudget)
	if onTime != 0 {
		t.Fatalf("%d bars marked late while every frame was inside the budget", onTime)
	}

	p.Render(frameCamera(), 40*time.Millisecond, Stats{})
	late := countFill(p.Render(frameCamera(), 8*time.Millisecond, Stats{}), perfOverBudget)
	if late == 0 {
		t.Fatal("a 40ms frame was not marked as over budget")
	}
}

func countFill(commands []render.Command, fill render.Color) int {
	count := 0
	for _, command := range commands {
		if rect, ok := command.(render.DrawRect); ok && rect.Fill == fill {
			count++
		}
	}
	return count
}
