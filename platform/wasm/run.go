//go:build js && wasm

package wasm

import (
	"fmt"
	"log/slog"
	"syscall/js"

	"github.com/dhannyell/frameforge/input"
	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
	"github.com/dhannyell/frameforge/runtime"
)

// Options configures the browser WASM host.
type Options struct {
	// CanvasID is the DOM canvas element id. Default "ff-canvas".
	CanvasID string
	// Log receives host diagnostics. Default slog.Default().
	Log *slog.Logger
	// ForceCanvas2D skips WebGPU initialization and uses the Canvas2D fallback.
	ForceCanvas2D bool
	// RecordReplay enables an in-browser recording/replay panel: start/stop
	// recording, download the .ffrp binary, and load a .ffrp to verify+replay.
	// Default false; production builds leave it off. Requires GameFactory.
	RecordReplay bool
	// GameFactory builds a fresh game instance for Verify/Replay. Replay
	// mutates its game argument, so each call needs a fresh one. Required when
	// RecordReplay is true; ignored otherwise. The engine never clones
	// arbitrary user types — the app owns construction.
	GameFactory func() runtime.Game
}

type host struct {
	driver *runtime.Driver
	canvas js.Value
	cfg    runtime.Config
	log    *slog.Logger

	// JS/WASM runs callbacks on the same thread; pending is drained by startLoop.
	pending []input.Event

	lastTimestamp float64
	hasTimestamp  bool
	renderer      renderer

	// panel is the optional recording/replay UI; nil when RecordReplay is off.
	panel *recordingPanel

	loopCb   js.Func
	resizeCb js.Func
	inputCbs []js.Func
}

// Run starts the browser WASM host for game and blocks forever.
func Run(game runtime.Game, opts Options) error {
	driver := runtime.NewDriver(game, opts.Log)
	return RunDriver(driver, game, opts)
}

// RunDriver starts the browser WASM host for an existing driver and blocks
// forever. It mirrors platform/web/canvas.RunDriver so an app that built its
// own driver (e.g. to load a .ffrp before serving) can still use the host.
func RunDriver(driver *runtime.Driver, game runtime.Game, opts Options) error {
	log := opts.Log
	if log == nil {
		log = slog.Default()
	}

	canvasID := opts.CanvasID
	if canvasID == "" {
		canvasID = "ff-canvas"
	}

	cfg := driver.Config()

	doc := js.Global().Get("document")
	canvas := doc.Call("getElementById", canvasID)
	if canvas.IsNull() {
		return fmt.Errorf("canvas element %q not found", canvasID)
	}

	h := &host{driver: driver, canvas: canvas, cfg: cfg, log: log}
	h.resize()
	h.registerResizeListener()
	h.registerInputListeners()
	h.renderer = selectRenderer(canvas, cfg.Clear, opts.ForceCanvas2D, log)
	if opts.RecordReplay {
		if opts.GameFactory == nil {
			log.Error("RecordReplay enabled without GameFactory; panel disabled")
		} else {
			h.panel = newRecordingPanel(h, opts.GameFactory, log)
		}
	}
	h.startLoop()
	select {}
}

func selectRenderer(canvas js.Value, clear render.Color, forceCanvas2D bool, log *slog.Logger) renderer {
	if forceCanvas2D {
		log.Info("using canvas2d renderer")
		return newCanvas2D(canvas, clear)
	}

	r, err := newWebGPU(canvas, clear)
	if err == nil {
		log.Info("using webgpu renderer")
		return r
	}

	log.Warn("webgpu unavailable, falling back to canvas2d", "error", err)
	return newCanvas2D(canvas, clear)
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
	h.driver.SetViewport(math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: w, Y: h_}))
}

func (h *host) registerResizeListener() {
	h.resizeCb = js.FuncOf(func(this js.Value, args []js.Value) any {
		h.resize()
		return nil
	})

	js.Global().Call("addEventListener", "resize", h.resizeCb)
}
