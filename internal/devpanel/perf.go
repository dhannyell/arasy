//go:build arr_dev

package devpanel

import (
	"math"
	"time"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
	"github.com/dhannyell/arasy/ui"
)

// The Perf tab exists because an average hides the thing that actually hurts.
// A run that drops four frames a second still reports a healthy mean, and the
// only symptom a developer can name is "it stutters sometimes" — which is how
// a garbage-collection problem is usually described before it is measured.
// The graph below shows every frame, so a spike is a spike instead of a
// rounding error in an average.

const (
	// frameHistoryLen is a couple of seconds at 60Hz: long enough to catch a
	// periodic spike, short enough that the bars stay wide enough to see.
	frameHistoryLen = 120

	perfGraphHeight = 58
	// perfCeilingFactor is how far above the frame budget the graph reaches.
	// Anything taller is clipped to the top — past twice the budget the exact
	// height stops mattering, the point is that it is bad.
	perfCeilingFactor = 3
	perfLineHeight    = 18
)

// frameHistory is a ring of recent frame durations, each flagged with whether
// the collector ran during it. The flag rides along with the sample so a bar
// and its cause cannot drift apart as the ring wraps.
type frameHistory struct {
	samples   [frameHistoryLen]time.Duration
	collected [frameHistoryLen]bool
	next      int
	filled    bool
}

func (h *frameHistory) push(frame time.Duration, collected bool) {
	if frame <= 0 {
		return
	}
	h.samples[h.next] = frame
	h.collected[h.next] = collected
	h.next = (h.next + 1) % len(h.samples)
	if h.next == 0 {
		h.filled = true
	}
}

// each walks the samples oldest to newest, so the graph scrolls left to right
// regardless of where the ring's cursor happens to sit.
func (h *frameHistory) each(visit func(index int, frame time.Duration, collected bool)) {
	count := h.next
	start := 0
	if h.filled {
		count = len(h.samples)
		start = h.next
	}
	for i := range count {
		slot := (start + i) % len(h.samples)
		visit(i, h.samples[slot], h.collected[slot])
	}
}

func (h *frameHistory) count() int {
	if h.filled {
		return len(h.samples)
	}
	return h.next
}

type perfLayout struct {
	graph math2d.Rect
	lines math2d.Rect
}

type perfBarMesh struct {
	vertices []math2d.Vec2
	indices  []uint32
	color    render.Color
}

func (m *perfBarMesh) reset(color render.Color) {
	m.vertices = m.vertices[:0]
	m.indices = m.indices[:0]
	m.color = color
}

func (m *perfBarMesh) appendRect(rect math2d.Rect) {
	base := uint32(len(m.vertices))
	m.vertices = append(m.vertices,
		rect.Min,
		math2d.Vec2{X: rect.Max.X, Y: rect.Min.Y},
		rect.Max,
		math2d.Vec2{X: rect.Min.X, Y: rect.Max.Y},
	)
	m.indices = append(m.indices,
		base, base+1, base+2,
		base, base+2, base+3,
	)
}

func perfRects(rect math2d.Rect) perfLayout {
	graph, rest := ui.CutTop(rect, perfGraphHeight)
	_, lines := ui.CutTop(rest, rowGap)
	return perfLayout{graph: graph, lines: lines}
}

// frameBudget is the frame time the host is aiming for, taken from the game's
// configured frame rate rather than assumed: a game running at 30 has a
// different idea of a bad frame than one running at 144.
func (p *Panel) frameBudget() time.Duration {
	fps := p.driver.Config().FPS
	if fps <= 0 {
		return time.Second / 60
	}
	return time.Second / time.Duration(fps)
}

func (p *Panel) drawPerf(draw ui.DrawScope, rect math2d.Rect) {
	lay := perfRects(rect)
	budget := p.frameBudget()

	p.drawFrameGraph(draw, lay.graph, budget)
	p.drawPerfLines(draw, lay.lines, budget)
}

func (p *Panel) drawFrameGraph(draw ui.DrawScope, rect math2d.Rect, budget time.Duration) {
	draw.FillRect(rect, p.theme.Progress.Background)

	ceiling := float64(budget) * perfCeilingFactor
	if ceiling <= 0 {
		return
	}

	// The budget line is what turns the bars into a judgement: below it the
	// frame was delivered on time, above it something was missed.
	budgetY := rect.Max.Y - rect.Height()*float64(budget)/ceiling
	draw.FillRect(math2d.NewRect(
		math2d.Vec2{X: rect.Min.X, Y: budgetY},
		math2d.Vec2{X: rect.Width(), Y: 1},
	), p.theme.Text.Muted)

	samples := p.history.count()
	if samples == 0 {
		return
	}
	width := rect.Width() / float64(len(p.history.samples))
	p.perfBars[0].reset(p.theme.Progress.Fill)
	p.perfBars[1].reset(colorWarning)
	p.perfBars[2].reset(colorCollected)

	p.history.each(func(index int, frame time.Duration, collected bool) {
		height := math.Min(float64(frame)/ceiling, 1) * rect.Height()
		if height <= 0 {
			return
		}
		mesh := &p.perfBars[0]
		if frame > budget {
			mesh = &p.perfBars[1]
		}
		// A collection outranks "late": a late frame asks why, and this is the
		// answer whenever it applies. An on-time frame that collected is worth
		// marking too — it is the same cost landing inside the budget.
		if collected {
			mesh = &p.perfBars[2]
		}
		mesh.appendRect(math2d.NewRect(
			math2d.Vec2{X: rect.Min.X + float64(index)*width, Y: rect.Max.Y - height},
			math2d.Vec2{X: math.Max(width-1, 1), Y: height},
		))
	})
	for i := range p.perfBars {
		mesh := &p.perfBars[i]
		if len(mesh.indices) > 0 {
			draw.DrawTriangles(mesh.vertices, mesh.indices, mesh.color)
		}
	}
}

func (p *Panel) drawPerfLines(draw ui.DrawScope, rect math2d.Rect, budget time.Duration) {
	frame := p.frameStats
	lines := []string{
		"fps " + formatFloat(frame.FPS) + "   frame " + formatMillis(frame.FrameTime),
		"mean " + formatMillis(frame.MeanFrameTime) +
			"   p99 " + formatMillis(frame.P99FrameTime) +
			"   budget " + formatMillis(budget),
	}

	// Host timings share one row: three short readings never justify three
	// rows of a panel this size, especially once the system rows join in.
	// The present wait is shown apart from CPU render work: with vsync it is
	// the vblank block, and reading it as render time makes an idle loop
	// look saturated.
	timings := ""
	if p.stats.CPUSet {
		timings = "cpu tick " + formatMillis(p.stats.CPUTick) +
			"   render " + formatMillis(p.stats.CPURender)
		if p.stats.PresentSet {
			timings += "   present " + formatMillis(p.stats.PresentWait)
		}
	}
	if p.stats.InputLatencySet {
		if timings != "" {
			timings += "   input " + formatMillis(p.stats.InputLatency)
		} else {
			timings = "input to submit " + formatMillis(p.stats.InputLatency)
		}
	}
	if timings != "" {
		lines = append(lines, timings)
	}

	heap := "commands " + formatInt(p.stats.Commands)
	if p.stats.HeapSet {
		heap = "heap " + formatMegabytes(p.stats.HeapInUse) +
			" / " + formatMegabytes(p.stats.HeapTotal) + "   " + heap
	}
	lines = append(lines, heap)

	// System rows, each present only when the host's probes could measure it.
	// Together they answer "is it the game or the machine?": the process
	// number is the game, the system number is everything else.
	if p.stats.MemSet {
		lines = append(lines, "mem "+formatMegabytes(p.stats.MemProcessBytes)+
			"   sys "+formatGigabytes(p.stats.MemSystemUsedBytes)+
			" / "+formatGigabytes(p.stats.MemSystemTotalBytes))
	}
	if p.stats.VRAMSet {
		lines = append(lines, "vram "+formatMegabytes(p.stats.VRAMProcessBytes)+
			" / "+formatGigabytes(p.stats.VRAMBudgetBytes))
	}
	if p.stats.GPUSet {
		lines = append(lines, "gpu "+formatPercent(p.stats.GPUProcessPercent)+
			"   sys "+formatPercent(p.stats.GPUSystemPercent))
	}
	// What the GPU spent, per pass. Kept apart from the utilization row above:
	// a busy percentage says the GPU had work, these say where the work went.
	if p.stats.GPUTimeSet {
		lines = append(lines, "gpu emit "+formatGPUTime(p.stats.GPUEmitTime)+
			"   sim "+formatGPUTime(p.stats.GPUSimulateTime)+
			"   draw "+formatGPUTime(p.stats.GPURenderTime))
	}
	// The allocation rate is the actionable half of this row: it is what sets
	// how often the collector has to run, and unlike the cycle count it is
	// something the game's code can change.
	if p.stats.GCSet {
		gc := "alloc " + formatMegabytes(p.stats.GCAllocBytes) +
			"   heap " + formatMegabytes(p.stats.GCHeapLive) +
			" / " + formatMegabytes(p.stats.GCHeapGoal)
		if p.stats.GCCycles > 0 {
			gc += "   gc " + formatInt(int(p.stats.GCCycles)) +
				" " + formatGPUTime(p.stats.GCPause)
		}
		lines = append(lines, gc)
	}

	for i, line := range lines {
		p.text(draw, line, math2d.Vec2{
			X: rect.Min.X,
			Y: rect.Min.Y + float64(i)*perfLineHeight + perfLineHeight/2,
		}, p.theme.Text.Color, p.theme.Text.Caption)
	}
}
