//go:build (windows || linux || darwin) && (amd64 || arm64)

package native

import (
	"fmt"
	"image"

	"github.com/go-gl/glfw/v3.4/glfw"
)

type windowRect struct {
	x, y          int
	width, height int
}

func createWindow(opts Options) (*glfw.Window, error) {
	glfw.DefaultWindowHints()
	glfw.WindowHint(glfw.ClientAPI, glfw.NoAPI)
	glfw.WindowHint(glfw.Resizable, glfw.True)
	glfw.WindowHint(glfw.Visible, glfw.False)
	glfw.WindowHint(glfw.Focused, glfw.False)

	if opts.Fullscreen {
		monitor, mode, err := primaryVideoMode()
		if err != nil {
			return nil, err
		}
		glfw.WindowHint(glfw.Decorated, glfw.False)
		glfw.WindowHint(glfw.RedBits, mode.RedBits)
		glfw.WindowHint(glfw.GreenBits, mode.GreenBits)
		glfw.WindowHint(glfw.BlueBits, mode.BlueBits)
		glfw.WindowHint(glfw.RefreshRate, mode.RefreshRate)
		window, err := glfw.CreateWindow(mode.Width, mode.Height, opts.Title, monitor, nil)
		if err != nil {
			return nil, fmt.Errorf("create fullscreen native window: %w", err)
		}
		setIcon(window, opts.Icon)
		return window, nil
	}

	window, err := glfw.CreateWindow(opts.Width, opts.Height, opts.Title, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("create native window: %w", err)
	}
	setIcon(window, opts.Icon)
	return window, nil
}

func setIcon(window *glfw.Window, icon image.Image) {
	if window != nil && icon != nil {
		window.SetIcon([]image.Image{icon})
	}
}

func primaryVideoMode() (*glfw.Monitor, *glfw.VidMode, error) {
	monitor := glfw.GetPrimaryMonitor()
	if monitor == nil {
		return nil, nil, fmt.Errorf("primary monitor is unavailable")
	}
	mode := monitor.GetVideoMode()
	if mode == nil {
		return nil, nil, fmt.Errorf("primary monitor has no video mode")
	}
	return monitor, mode, nil
}

func toggleFullscreen(window *glfw.Window, saved *windowRect) {
	if window == nil || saved == nil {
		return
	}
	if window.GetMonitor() == nil {
		x, y := window.GetPos()
		width, height := window.GetSize()
		*saved = windowRect{x: x, y: y, width: width, height: height}

		monitor, mode, err := primaryVideoMode()
		if err != nil {
			return
		}
		window.SetAttrib(glfw.Decorated, glfw.False)
		window.SetMonitor(monitor, 0, 0, mode.Width, mode.Height, mode.RefreshRate)
		return
	}

	window.SetMonitor(nil, saved.x, saved.y, saved.width, saved.height, 0)
	window.SetAttrib(glfw.Decorated, glfw.True)
}
