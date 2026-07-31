//go:build js && wasm

package wasm

import (
	"fmt"
	"log/slog"
	"syscall/js"

	"github.com/dhannyell/arasy/asset"
	"github.com/dhannyell/arasy/input"
	"github.com/dhannyell/arasy/internal/platforminput"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
	"github.com/dhannyell/arasy/runtime"
)

// Options configures the browser WASM host.
type Options struct {
	// CanvasID is the DOM canvas element id. Default "ar-canvas".
	CanvasID string
	// Log receives host diagnostics. Default slog.Default().
	Log *slog.Logger
	// ForceCanvas2D skips WebGPU initialization and uses the Canvas2D fallback.
	ForceCanvas2D bool
	// DevTools enables the in-browser debug control panel. Default false; the
	// panel starts hidden and is toggled with the Backquote/~ key. The panel
	// only exists in builds tagged arr_dev (arasy dev adds it); in any other
	// build this option is inert.
	DevTools bool
	// RecordReplay enables an in-browser recording/replay panel: start/stop
	// recording, download the .arrp binary, and load a .arrp to verify+replay.
	// Default false; production builds leave it off. Requires GameFactory.
	RecordReplay bool
	// GameFactory builds a fresh game instance for Verify/Replay. Replay
	// mutates its game argument, so each call needs a fresh one. Required when
	// RecordReplay is true; ignored otherwise. The engine never clones
	// arbitrary user types — the app owns construction.
	GameFactory func() runtime.Game
	// MSAA selects the antialiasing level of the WebGPU renderer. The zero
	// value is render.MSAADefault (4x); render.MSAAOff turns antialiasing
	// off. The Canvas2D fallback ignores it.
	MSAA render.MSAAMode
	// FrameMetrics receives lightweight browser frame telemetry. The callback
	// runs at most four times per second after a frame has been submitted.
	// InputToSubmit measures browser-event arrival to render submission; it is
	// not a display scan-out (input-to-photon) measurement.
	FrameMetrics func(FrameMetrics)
}

type host struct {
	driver *runtime.Driver
	canvas js.Value
	cfg    runtime.Config
	log    *slog.Logger

	// JS/WASM runs callbacks on the same thread; pending is drained by startLoop.
	pending              []input.Event
	pendingInputAtMillis float64
	frameMetrics         frameMetricTracker
	onFrameMetrics       func(FrameMetrics)

	renderer runtime.FrameRenderer
	// viewport mirrors the canvas size the driver was given, so the dev panel
	// can lay itself out in the same pixels the frame is drawn in.
	viewport math2d.Rect

	// panel is the optional tooling overlay; nil in builds without the
	// arr_dev tag, where the panel is not compiled at all. It draws inside
	// the canvas like everything else, so there is no DOM overlay to keep
	// in sync with the frame.
	panel      devOverlay
	panelStats overlayStats
	audio      *audioSink

	loopCb   js.Func
	resizeCb js.Func
	inputCbs []js.Func

	gamepadTracker        platforminput.GamepadTracker
	gamepadSelector       platforminput.StickyGamepadSelector
	gamepadSupportScratch []bool
	touch                 platforminput.SingleTouch
}

// Run starts the browser WASM host for game and blocks forever.
func Run(game runtime.Game, opts Options) error {
	driver := runtime.NewDriver(game, opts.Log)
	return RunDriver(driver, game, opts)
}

// RunDriver starts the browser WASM host for an existing driver and blocks
// forever. It mirrors platform/web/canvas.RunDriver so an app that built its
// own driver (e.g. to load a .arrp before serving) can still use the host.
func RunDriver(driver *runtime.Driver, game runtime.Game, opts Options) error {
	applyBrowserSplashPolicy(driver)
	log := opts.Log
	if log == nil {
		log = slog.Default()
	}

	canvasID := opts.CanvasID
	if canvasID == "" {
		canvasID = "ar-canvas"
	}

	cfg := driver.Config()

	doc := js.Global().Get("document")
	canvas := doc.Call("getElementById", canvasID)
	if canvas.IsNull() {
		return fmt.Errorf("canvas element %q not found", canvasID)
	}

	h := &host{
		driver:               driver,
		canvas:               canvas,
		cfg:                  cfg,
		log:                  log,
		pendingInputAtMillis: -1,
		onFrameMetrics:       opts.FrameMetrics,
	}
	h.resize()
	h.registerResizeListener()
	h.registerInputListeners()
	if len(cfg.Assets.AudioClips()) > 0 {
		h.audio = newAudioSink(cfg.Assets.AudioClips(), cfg.Assets.FS(), cfg.SimulationHz, log)
	}
	h.renderer = selectRenderer(canvas, cfg.Clear, cfg.Assets, opts.ForceCanvas2D, opts.FrameMetrics != nil, cfg.ParticleOffscreenScale, opts.MSAA, log)
	var replayFactory func() runtime.Game
	if opts.RecordReplay {
		if opts.GameFactory == nil {
			log.Error("RecordReplay enabled without GameFactory; recording controls disabled")
		} else {
			replayFactory = opts.GameFactory
		}
	}
	if opts.DevTools || replayFactory != nil {
		h.mountDevPanel(opts.DevTools, replayFactory, log)
	}
	h.startLoop()
	select {}
}

func applyBrowserSplashPolicy(driver *runtime.Driver) {
	policy := js.Global().Get("arasySplashPolicy")
	if policy.Type() == js.TypeObject {
		game := policy.Get("game")
		if game.Type() == js.TypeBoolean && !game.Bool() {
			driver.SkipSplash()
			return
		}
		engine := policy.Get("engine")
		if engine.Type() != js.TypeBoolean || !engine.Bool() {
			driver.SkipDefaultSplash()
		}
		return
	}

	// Compatibility with shells generated before loading and runtime splash
	// became independent policies.
	if disabled := js.Global().Get("arasyDisableSplash"); disabled.Type() == js.TypeBoolean && disabled.Bool() {
		driver.SkipSplash()
	}
}

func selectRenderer(canvas js.Value, clear render.Color, manifest asset.Manifest, forceCanvas2D, wantGPUTiming bool, particleScale float64, msaa render.MSAAMode, log *slog.Logger) runtime.FrameRenderer {
	if forceCanvas2D {
		log.Info("using canvas2d renderer")
		return newCanvas2D(canvas, clear, manifest, log)
	}

	r, err := newWebGPU(canvas, clear, manifest, wantGPUTiming, particleScale, msaa, log)
	if err == nil {
		log.Info("using webgpu renderer")
		return r
	}

	log.Warn("webgpu unavailable, falling back to canvas2d", "error", err)
	return newCanvas2D(canvas, clear, manifest, log)
}

func (h *host) resize() {
	parent := h.canvas.Get("parentElement")
	rect := parent.Call("getBoundingClientRect")
	dpr := js.Global().Get("devicePixelRatio").Float()

	if dpr <= 0 {
		dpr = 1
	}

	w := rect.Get("width").Float() * dpr
	h_ := rect.Get("height").Float() * dpr
	h.canvas.Set("width", w)
	h.canvas.Set("height", h_)
	h.viewport = math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: w, Y: h_})
	h.driver.SetViewport(h.viewport)
}

func (h *host) registerResizeListener() {
	h.resizeCb = js.FuncOf(func(this js.Value, args []js.Value) any {
		h.resize()
		return nil
	})

	js.Global().Call("addEventListener", "resize", h.resizeCb)
}
