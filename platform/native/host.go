//go:build windows && amd64

package native

import (
	"fmt"
	"log/slog"
	"time"

	metrics "runtime/metrics"

	"github.com/dhannyell/arasy/input"
	"github.com/dhannyell/arasy/internal/devpanel"
	"github.com/dhannyell/arasy/internal/platforminput"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/runtime"
	"github.com/go-gl/glfw/v3.4/glfw"
)

type cpuPhaseReporter interface {
	renderCPUTimings() (tess, upload, encode time.Duration, ok bool)
}

type gpuBufferReporter interface {
	particleGPUBufferBytes() (bytes uint64, ok bool)
}

const noGamepadJoystick glfw.Joystick = -1

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

	pending  []input.Event
	viewport math2d.Rect
	fbDirty  bool

	gamepadJoystick glfw.Joystick
	gamepadTracker  platforminput.GamepadTracker

	savedWindowed windowRect
	altEnter      bool

	// panel is the optional dev tooling overlay; nil unless the app asked for
	// it. It draws through the ordinary renderer, so there is no second
	// window and nothing platform-specific about it beyond the file I/O.
	panel *devpanel.Panel
	fps   fpsCounter

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
	return h
}

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
	h.panel = devpanel.New(h.driver, panelOpts)
}

func (h *host) noteInput() {
	if h.lastInputAt.IsZero() {
		h.lastInputAt = time.Now()
	}
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
		x, y := h.scaledCursor()
		h.noteInput()
		h.pending = append(h.pending, mouseButtonEvent(code, action == glfw.Press, x, y))
	})
	h.window.SetCursorPosCallback(func(_ *glfw.Window, x, y float64) {
		x, y = h.scaleCursor(x, y)
		h.noteInput()
		h.pending = append(h.pending, pointerMoveEvent(x, y))
	})
	h.window.SetScrollCallback(func(_ *glfw.Window, dx, dy float64) {
		x, y := h.scaledCursor()
		events := wheelEvents(dx, dy, x, y)
		if len(events) > 0 {
			h.noteInput()
			h.pending = append(h.pending, events...)
		}
	})
	h.window.SetCursorEnterCallback(func(_ *glfw.Window, entered bool) {
		if !entered {
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
		budget:     frameBudget(h.driver.Config().FPS),
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
		var tickStart time.Time
		if h.metrics != nil {
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
		if h.metrics != nil {
			tickEnd = time.Now()
		}
		h.renderFrame(elapsed)
		renderEnd := time.Now()
		if err := h.renderer.Err(); err != nil {
			return fmt.Errorf("native renderer failed: %w", err)
		}
		showWindowAfterPresent()
		if h.metrics != nil {
			if stats, ok := h.metrics.Observe(elapsed); ok {
				m := FrameMetrics{
					FPS: stats.FPS, FrameTime: stats.FrameTime,
					MeanFrameTime: stats.MeanFrameTime, P99FrameTime: stats.P99FrameTime,
					CPUTickTime: tickEnd.Sub(tickStart), CPURenderTime: renderEnd.Sub(tickEnd), CPUTimingSet: true,
				}
				if !inputAt.IsZero() {
					m.InputToSubmit = renderEnd.Sub(inputAt)
					m.InputToSubmitSet = true
				}
				m.HeapInUse, m.HeapSys, m.HeapSet = h.readHeap()
				if reporter, ok := any(h.renderer).(cpuPhaseReporter); ok {
					m.CPUTessTime, m.CPUUploadTime, m.CPUEncodeTime, m.CPUBreakdownSet = reporter.renderCPUTimings()
				}
				if reporter, ok := any(h.renderer).(gpuBufferReporter); ok {
					m.GPUBufferBytes, m.GPUBufferSet = reporter.particleGPUBufferBytes()
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
	stats := devpanel.Stats{FPS: h.fps.observe(elapsed), Commands: len(frame.Commands)}
	frame.Commands = append(frame.Commands, h.panel.Render(frame.Camera, elapsed, stats)...)
	h.renderer.UpdateResources(h.driver.ConsumeFrameUpdates())
	h.renderer.Render(frame)
}

// fpsCounter is a one-second frame count for the panel footer. The host's
// FrameStatsTracker would do the same and more, but it only exists when the
// app asked for metrics, and the panel should report a rate either way.
type fpsCounter struct {
	frames  int
	elapsed time.Duration
	rate    float64
}

func (c *fpsCounter) observe(elapsed time.Duration) float64 {
	c.frames++
	c.elapsed += elapsed
	if c.elapsed >= time.Second {
		c.rate = float64(c.frames) / c.elapsed.Seconds()
		c.frames = 0
		c.elapsed = 0
	}
	return c.rate
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
