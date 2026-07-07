//go:build js && wasm

package wasm

import (
	"fmt"
	"log/slog"
	"syscall/js"

	"github.com/dhannyell/frameforge/input"
	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/runtime"
)

type Options struct {
	// CanvasID is the canvas element <canvas> in DOM. Default "ff-canvas"
	CanvasID string
	Log      *slog.Logger
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
	renderer      *Canvas2D

	loopCb   js.Func
	resizeCb js.Func
	inputCbs []js.Func
}

func Run(game runtime.Game, opts Options) error {
	log := opts.Log
	if log == nil {
		log = slog.Default()
	}

	canvasID := opts.CanvasID
	if canvasID == "" {
		canvasID = "ff-canvas"
	}

	driver := runtime.NewDriver(game, log)
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
	h.renderer = newCanvas2D(canvas, cfg.Clear)
	h.startLoop()
	select {}
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
