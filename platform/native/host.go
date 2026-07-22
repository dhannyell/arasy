//go:build windows && amd64

package native

import (
	"fmt"
	"time"

	"github.com/dhannyell/frameforge/input"
	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/runtime"
	"github.com/go-gl/glfw/v3.4/glfw"
)

type host struct {
	driver   *runtime.Driver
	renderer *Renderer
	window   *glfw.Window
	audio    *audioSink

	pending  []input.Event
	viewport math2d.Rect
	fbDirty  bool

	savedWindowed windowRect
	altEnter      bool
}

func newHost(driver *runtime.Driver, renderer *Renderer, window *glfw.Window, opts Options) *host {
	fbW, fbH := window.GetFramebufferSize()
	return &host{
		driver:   driver,
		renderer: renderer,
		window:   window,
		viewport: viewportForFramebuffer(fbW, fbH),
		savedWindowed: windowRect{
			width:  opts.Width,
			height: opts.Height,
		},
	}
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
		code, ok := translateKey(key)
		if !ok {
			return
		}
		h.pending = append(h.pending, keyEvent(code, action == glfw.Press))
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
		h.pending = append(h.pending, mouseButtonEvent(code, action == glfw.Press, x, y))
	})
	h.window.SetCursorPosCallback(func(_ *glfw.Window, x, y float64) {
		x, y = h.scaleCursor(x, y)
		h.pending = append(h.pending, pointerMoveEvent(x, y))
	})
	h.window.SetScrollCallback(func(_ *glfw.Window, dx, dy float64) {
		x, y := h.scaledCursor()
		h.pending = append(h.pending, wheelEvents(dx, dy, x, y)...)
	})
	h.window.SetCursorEnterCallback(func(_ *glfw.Window, entered bool) {
		if !entered {
			h.pending = append(h.pending, pointerLeaveEvent())
		}
	})

	// -- Window

	h.window.SetFocusCallback(func(_ *glfw.Window, focused bool) {
		if focused {
			h.audio.resume()
		} else {
			h.audio.suspend()
		}
	})
}

func (h *host) run() error {
	var frameTimer runtime.FrameTimer
	for !h.window.ShouldClose() {
		glfw.PollEvents()
		h.resizeIfNeeded()

		elapsed := frameTimer.Elapsed(time.Now())
		_, audioOut := h.driver.RunTick(runtime.TickBatch{
			Viewport: &h.viewport,
			Events:   h.pending,
		}, elapsed)
		h.pending = h.pending[:0]

		if h.audio != nil {
			h.audio.consume(audioOut)
		}

		h.driver.RenderFrame(elapsed, h.renderer)
		if err := h.renderer.Err(); err != nil {
			return fmt.Errorf("native renderer failed: %w", err)
		}
	}
	return nil
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
