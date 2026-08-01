//go:build (windows || linux || darwin) && (amd64 || arm64)

package native

import (
	"fmt"
	"log/slog"
	"time"

	metrics "runtime/metrics"

	"github.com/dhannyell/arasy/input"
	"github.com/dhannyell/arasy/internal/platforminput"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/platform/native/internal/sysprobe"
	"github.com/dhannyell/arasy/runtime"
	"github.com/go-gl/glfw/v3.4/glfw"
)

type cpuPhaseReporter interface {
	renderCPUTimings() (tess, upload, encode time.Duration, ok bool)
}

type gpuBufferReporter interface {
	particleGPUBufferBytes() (bytes uint64, ok bool)
}

type gpuTimingReporter interface {
	particleGPUTimings() (prepare, simulate, render time.Duration, ok bool)
}

// presentWaitReporter exposes the presentation-queue block time of the last
// frame (acquire + present), which with vsync is the vblank wait and must
// not be read as CPU render work.
type presentWaitReporter interface {
	presentWaitDuration() (time.Duration, bool)
}

// cpuFrameTiming separates CPU work, surface acquisition and queue submission.
type cpuFrameTiming struct {
	submittedAt time.Time
	cpuEndAt    time.Time
	acquireWait time.Duration
	valid       bool
}

func (t cpuFrameTiming) renderDurationSince(start time.Time) time.Duration {
	if !t.valid || start.IsZero() {
		return 0
	}
	duration := t.cpuEndAt.Sub(start) - t.acquireWait
	if duration < 0 {
		return 0
	}
	return duration
}

func (t cpuFrameTiming) inputToSubmit(inputAt time.Time) (time.Duration, bool) {
	if !t.valid || inputAt.IsZero() {
		return 0, false
	}
	duration := t.submittedAt.Sub(inputAt)
	if duration < 0 {
		return 0, false
	}
	return duration, true
}

const noGamepadJoystick glfw.Joystick = -1

const panelGCSampleFrames = 8 // Per-frame runtime metrics materially slow the panel.

type host struct {
	driver      *runtime.Driver
	renderer    *Renderer
	window      *glfw.Window
	audio       *audioSink
	metrics     *runtime.FrameStatsTracker
	onMetrics   func(FrameMetrics)
	log         *slog.Logger
	logMetrics  bool
	lastInputAt time.Time
	heapSamples []metrics.Sample

	pending     []input.Event
	viewport    math2d.Rect
	fbDirty     bool
	cursorX     float64
	cursorY     float64
	cursorDirty bool

	gamepadJoystick glfw.Joystick
	gamepadTracker  platforminput.GamepadTracker

	savedWindowed windowRect
	altEnter      bool
	vsync         VSyncMode

	// panel is the optional tooling overlay; nil in builds without the
	// arr_dev tag, where the panel is not compiled at all. It draws through
	// the ordinary renderer, so there is no second window and nothing
	// platform-specific about it beyond the file I/O.
	panel      devOverlay
	panelStats overlayStats

	// sysprobe feeds the panel's system rows (process vs system RAM, VRAM,
	// GPU). Cheap to keep always: it samples once a second and only when the
	// panel asks.
	sysprobe *sysprobe.Prober

	gc       runtime.GCStatsTracker
	gcFrames uint8

	spinMargin time.Duration
}

func newHost(driver *runtime.Driver, renderer *Renderer, window *glfw.Window, opts Options) *host {
	fbW, fbH := window.GetFramebufferSize()
	h := &host{
		driver:          driver,
		renderer:        renderer,
		window:          window,
		log:             opts.Log,
		onMetrics:       opts.FrameMetrics,
		logMetrics:      opts.Metrics,
		viewport:        viewportForFramebuffer(fbW, fbH),
		gamepadJoystick: noGamepadJoystick,
		savedWindowed: windowRect{
			width:  opts.Width,
			height: opts.Height,
		},
		vsync: opts.VSync,
	}
	if opts.Metrics || opts.FrameMetrics != nil {
		h.metrics = &runtime.FrameStatsTracker{}
		h.heapSamples = []metrics.Sample{
			{Name: "/memory/classes/heap/objects:bytes"},
			{Name: "/memory/classes/total:bytes"},
		}
	}
	if opts.SmoothPacing {
		h.spinMargin = time.Millisecond
	}
	vendor, device := renderer.adapterIDs()
	h.sysprobe = sysprobe.NewProber(vendor, device)
	return h
}

// mountDevPanel lives in devtools_dev.go (arr_dev builds) and
// devtools_release.go (every other build, where it only warns).

func (h *host) noteInput() {
	if h.lastInputAt.IsZero() {
		h.lastInputAt = time.Now()
	}
}

func (h *host) queuePointerMove(x, y float64) {
	event := pointerMoveEvent(x, y)
	if last := len(h.pending) - 1; last >= 0 &&
		h.pending[last].Kind == input.EventPointerMove && h.pending[last].Source == event.Source {
		h.pending[last] = event
		return
	}
	h.pending = append(h.pending, event)
}

func (h *host) flushCursorMove() {
	if !h.cursorDirty {
		return
	}
	h.cursorDirty = false
	x, y := h.scaleCursor(h.cursorX, h.cursorY)
	h.queuePointerMove(x, y)
}

func (h *host) readHeap() (uint64, uint64, bool) {
	if len(h.heapSamples) == 0 {
		return 0, 0, false
	}
	metrics.Read(h.heapSamples)
	if h.heapSamples[0].Value.Kind() != metrics.KindUint64 || h.heapSamples[1].Value.Kind() != metrics.KindUint64 {
		return 0, 0, false
	}
	return h.heapSamples[0].Value.Uint64(), h.heapSamples[1].Value.Uint64(), true
}

func (h *host) registerCallbacks() {
	h.window.SetFramebufferSizeCallback(func(_ *glfw.Window, _, _ int) {
		h.fbDirty = true
	})

	// keyboard and mouse
	h.window.SetKeyCallback(func(_ *glfw.Window, key glfw.Key, _ int, action glfw.Action, mods glfw.ModifierKey) {
		if key == glfw.KeyEnter && action == glfw.Press && mods&glfw.ModAlt != 0 {
			toggleFullscreen(h.window, &h.savedWindowed)
			h.altEnter = true
			return
		}
		if key == glfw.KeyEnter && action == glfw.Release && h.altEnter {
			h.altEnter = false
			return
		}
		if action == glfw.Repeat {
			return
		}
		// The overlay key is host chrome, like Alt+Enter above it: it never
		// reaches the input vocabulary, so no game can bind it and no
		// recording can contain it.
		if key == glfw.KeyGraveAccent && action == glfw.Press && h.panel != nil {
			h.panel.Toggle()
			return
		}
		code, ok := translateKey(key)
		if !ok {
			return
		}
		h.flushCursorMove()
		h.noteInput()
		h.pending = append(h.pending, keyEvent(code, action == glfw.Press))
	})
	// GLFW resolves layout, modifiers and IME for us and delivers finished
	// codepoints here, repeats included — the same characters the browser
	// path derives from KeyboardEvent.key.
	h.window.SetCharCallback(func(_ *glfw.Window, char rune) {
		// GLFW reports the overlay key as a character too. Dropping it keeps
		// the key from being typed into the panel it just opened.
		if h.panel != nil && (char == '`' || char == '~') {
			return
		}
		event, ok := input.TextEvent(char)
		if !ok {
			return
		}
		h.flushCursorMove()
		h.noteInput()
		h.pending = append(h.pending, event)
	})
	h.window.SetMouseButtonCallback(func(_ *glfw.Window, button glfw.MouseButton, action glfw.Action, _ glfw.ModifierKey) {
		if action != glfw.Press && action != glfw.Release {
			return
		}
		code, ok := translateMouseButton(button)
		if !ok {
			return
		}
		h.flushCursorMove()
		x, y := h.scaledCursor()
		h.noteInput()
		h.pending = append(h.pending, mouseButtonEvent(code, action == glfw.Press, x, y))
	})
	h.window.SetCursorPosCallback(func(_ *glfw.Window, x, y float64) {
		h.cursorX, h.cursorY = x, y
		h.cursorDirty = true
		h.noteInput()
	})
	h.window.SetScrollCallback(func(_ *glfw.Window, dx, dy float64) {
		h.flushCursorMove()
		x, y := h.scaledCursor()
		events := wheelEvents(dx, dy, x, y)
		if len(events) > 0 {
			h.noteInput()
			h.pending = append(h.pending, events...)
		}
	})
	h.window.SetCursorEnterCallback(func(_ *glfw.Window, entered bool) {
		if !entered {
			h.flushCursorMove()
			h.noteInput()
			h.pending = append(h.pending, pointerLeaveEvent())
		}
	})

	// -- Window

	h.window.SetFocusCallback(func(_ *glfw.Window, focused bool) {
		h.audio.setFocused(focused)
	})
}

func (h *host) run() error {
	var frameTimer runtime.FrameTimer
	pacer := framePacer{
		budget:     pacerBudget(h.driver.Config().FPS, h.vsync),
		spinMargin: h.spinMargin,
	}
	windowShown := false
	showWindowAfterPresent := func() {
		if windowShown || !h.renderer.hasPresentedFrame() {
			return
		}
		h.window.Show()
		h.window.Focus()
		windowShown = true
	}
	for !h.window.ShouldClose() {
		glfw.PollEvents()
		h.flushCursorMove()
		h.resizeIfNeeded()

		elapsed := frameTimer.Elapsed(time.Now())
		if gamepadEvents := h.pollGamepad(); len(gamepadEvents) > 0 {
			h.noteInput()
			h.pending = append(h.pending, gamepadEvents...)
		}

		splash := h.driver.RunSplashFrame(runtime.SplashFrameBatch{
			Viewport: &h.viewport,
			Events:   h.pending,
		}, elapsed, h.renderer)
		if splash.Rendered {
			h.pending = h.pending[:0]
			h.lastInputAt = time.Time{}
			if err := h.renderer.Err(); err != nil {
				return fmt.Errorf("splash renderer failed: %w", err)
			}
			showWindowAfterPresent()
			pacer.wait()
			continue
		}

		inputAt := h.lastInputAt
		h.lastInputAt = time.Time{}
		// The dev panel reports the same split the metrics reporter does, so
		// an app that never asked for metrics still gets a Perf tab.
		sampleCPU := h.metrics != nil || h.panelWantsStats()
		var tickStart time.Time
		if sampleCPU {
			tickStart = time.Now()
		}
		// The panel sees every event first and returns the ones the game
		// should still receive; what it kept never reaches the driver, and so
		// never enters a recording.
		events := h.pending
		if h.panel != nil {
			events = h.panel.HandleInput(events, h.viewport)
		}
		_, audioOut := h.driver.RunTick(runtime.TickBatch{
			Viewport: &h.viewport,
			Events:   events,
		}, elapsed)
		h.pending = h.pending[:0]

		if h.audio != nil {
			h.audio.setAudioContextState(h.driver.State().Paused)
			h.audio.consume(audioOut)
		}

		var tickEnd time.Time
		if sampleCPU {
			tickEnd = time.Now()
		}
		if h.panel != nil {
			h.samplePanelGC()
		}
		h.renderFrame(elapsed)
		cpuFrame := h.renderer.frameCPUTiming()
		if h.panelWantsStats() {
			h.samplePanelStats(tickStart, tickEnd, inputAt, cpuFrame)
		}
		if err := h.renderer.Err(); err != nil {
			return fmt.Errorf("native renderer failed: %w", err)
		}
		showWindowAfterPresent()
		if h.metrics != nil {
			if stats, ok := h.metrics.Observe(elapsed); ok {
				m := FrameMetrics{
					FPS: stats.FPS, FrameTime: stats.FrameTime,
					MeanFrameTime: stats.MeanFrameTime, P99FrameTime: stats.P99FrameTime,
				}
				if cpuFrame.valid {
					m.CPUTickTime = tickEnd.Sub(tickStart)
					m.CPURenderTime = cpuFrame.renderDurationSince(tickEnd)
					m.CPUTimingSet = true
				}
				if latency, ok := cpuFrame.inputToSubmit(inputAt); ok {
					m.InputToSubmit = latency
					m.InputToSubmitSet = true
				}
				m.HeapInUse, m.HeapSys, m.HeapSet = h.readHeap()
				if reporter, ok := any(h.renderer).(cpuPhaseReporter); ok {
					m.CPUTessTime, m.CPUUploadTime, m.CPUEncodeTime, m.CPUBreakdownSet = reporter.renderCPUTimings()
				}
				if reporter, ok := any(h.renderer).(gpuBufferReporter); ok {
					m.GPUBufferBytes, m.GPUBufferSet = reporter.particleGPUBufferBytes()
				}
				if reporter, ok := any(h.renderer).(gpuTimingReporter); ok {
					m.GPUPrepareTime, m.GPUSimulateTime, m.GPURenderTime, m.GPUTimingSet = reporter.particleGPUTimings()
				}
				if h.logMetrics {
					attrs := []any{"fps", m.FPS, "frame_ms", m.FrameTime.Seconds() * 1000, "mean_ms", m.MeanFrameTime.Seconds() * 1000, "p99_ms", m.P99FrameTime.Seconds() * 1000}
					h.log.Info("native frame metrics", attrs...)
				}
				if h.onMetrics != nil {
					h.onMetrics(m)
				}
			}
		}

		pacer.wait()
	}
	return nil
}

// renderFrame is runtime.RenderFrame with the dev panel spliced in: its
// commands go after the game's, in a layer band above every game layer. With
// the panel closed or absent this is the frame the driver would have rendered
// on its own, command for command.
func (h *host) renderFrame(elapsed time.Duration) {
	if h.panel == nil {
		h.driver.RenderFrame(elapsed, h.renderer)
		return
	}

	frame := h.driver.Frame(elapsed)
	h.panelStats.Commands = len(frame.Commands)
	frame.Commands = append(frame.Commands, h.panel.Render(frame.Camera, elapsed, h.panelStats)...)
	h.renderer.UpdateResources(h.driver.ConsumeFrameUpdates())
	h.renderer.Render(frame)
}

// panelWantsStats reports whether the dev panel is open and therefore worth
// measuring for.
func (h *host) panelWantsStats() bool {
	return h.panel != nil && h.panel.Visible()
}

// samplePanelStats fills in the numbers the panel cannot measure for itself.
// They describe the frame just submitted, so the panel shows them one frame
// late: the render time of the frame the panel is being drawn into is not
// known until after it has been drawn.
func (h *host) samplePanelStats(tickStart, tickEnd, inputAt time.Time, cpuFrame cpuFrameTiming) {
	h.panelStats.CPUSet = cpuFrame.valid
	if h.panelStats.CPUSet {
		h.panelStats.CPUTick = tickEnd.Sub(tickStart)
		h.panelStats.CPURender = cpuFrame.renderDurationSince(tickEnd)
	}

	h.panelStats.InputLatency, h.panelStats.InputLatencySet = cpuFrame.inputToSubmit(inputAt)
	if !h.panelStats.InputLatencySet {
		h.panelStats.InputLatency = 0
	}

	if reporter, ok := any(h.renderer).(gpuTimingReporter); ok {
		h.panelStats.GPUEmitTime, h.panelStats.GPUSimulateTime,
			h.panelStats.GPURenderTime, h.panelStats.GPUTimeSet = reporter.particleGPUTimings()
	}

	if reporter, ok := any(h.renderer).(presentWaitReporter); ok {
		h.panelStats.PresentWait, h.panelStats.PresentSet = reporter.presentWaitDuration()
	}

	probe := h.sysprobe.Sample(time.Now())
	h.panelStats.MemProcessBytes = probe.MemProcessBytes
	h.panelStats.MemSystemUsedBytes = probe.MemSystemUsedBytes
	h.panelStats.MemSystemTotalBytes = probe.MemSystemTotalBytes
	h.panelStats.MemSet = probe.MemOK
	h.panelStats.VRAMProcessBytes = probe.VRAMProcessBytes
	h.panelStats.VRAMBudgetBytes = probe.VRAMBudgetBytes
	h.panelStats.VRAMSet = probe.VRAMOK
	h.panelStats.GPUProcessPercent = probe.GPUProcessPercent
	h.panelStats.GPUSystemPercent = probe.GPUSystemPercent
	h.panelStats.GPUSet = probe.GPUOK
}

func (h *host) samplePanelGC() {
	h.panelStats.GCFresh = false
	h.gcFrames++
	if h.gcFrames < panelGCSampleFrames {
		return
	}
	h.gcFrames = 0
	if h.panelWantsStats() {
		h.panelStats.HeapInUse, h.panelStats.HeapTotal, h.panelStats.HeapSet = h.readHeap()
	}

	gc, ok := h.gc.Sample()
	h.panelStats.GCSet = ok
	if !ok {
		h.panelStats.GCCycles = 0
		h.panelStats.GCPause = 0
		h.panelStats.GCAllocBytes = 0
		h.panelStats.GCHeapLive = 0
		h.panelStats.GCHeapGoal = 0
		return
	}
	h.panelStats.GCCycles, h.panelStats.GCPause = gc.Cycles, gc.Pause
	h.panelStats.GCAllocBytes = gc.AllocBytes
	h.panelStats.GCHeapLive, h.panelStats.GCHeapGoal = gc.HeapLive, gc.HeapGoal
	h.panelStats.GCFresh = true
}

func (h *host) pollGamepad() []input.Event {
	// Poll the first mapped GLFW device and emit input.DeviceGamepad events.
	if h.gamepadJoystick == noGamepadJoystick {
		for joystick := glfw.Joystick1; joystick <= glfw.Joystick16; joystick++ {
			if joystick.Present() && joystick.IsGamepad() {
				h.gamepadJoystick = joystick
				break
			}
		}
	}

	if h.gamepadJoystick == noGamepadJoystick {
		h.driver.SetGamepadConnected(false)
		return nil
	}

	state := h.gamepadJoystick.GetGamepadState()
	if state == nil {
		h.driver.SetGamepadConnected(false)
		events := h.gamepadTracker.Update(false, platforminput.GamepadSnapshot{})
		h.gamepadJoystick = noGamepadJoystick
		return events
	}
	h.driver.SetGamepadConnected(true)

	var buttons [15]bool
	for i := range buttons {
		buttons[i] = state.Buttons[i] == glfw.Press
	}
	var axes [6]float64
	for i := range axes {
		axes[i] = float64(state.Axes[i])
	}
	current := platforminput.GLFWGamepadSnapshot(buttons, axes)
	return h.gamepadTracker.Update(true, current)
}

func (h *host) resizeIfNeeded() {
	if !h.fbDirty {
		return
	}
	fbW, fbH := h.window.GetFramebufferSize()
	h.renderer.Resize(fbW, fbH)
	h.viewport = viewportForFramebuffer(fbW, fbH)
	h.fbDirty = false
}

func (h *host) scaledCursor() (float64, float64) {
	x, y := h.window.GetCursorPos()
	return h.scaleCursor(x, y)
}

func (h *host) scaleCursor(x, y float64) (float64, float64) {
	winW, winH := h.window.GetSize()
	fbW, fbH := h.window.GetFramebufferSize()
	return scaleCursor(x, y, winW, winH, fbW, fbH)
}

func viewportForFramebuffer(width, height int) math2d.Rect {
	return math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: float64(width), Y: float64(height)})
}
