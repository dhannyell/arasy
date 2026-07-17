package runtime

import (
	"time"

	"github.com/dhannyell/frameforge/render"
)

type Frame struct {
	Camera   render.Camera
	Commands []render.Command
}

// FrameRenderer is what a host renderer must implement to receive one frame:
// first the dynamic-texture updates drained from the driver, then the frame
// itself. Both wasm renderers and the raster target satisfy it.
type FrameRenderer interface {
	UpdateTextures(updates []TextureUpdate)
	Render(frame Frame)
}

// RenderFrame assembles the frame, drains pending dynamic-texture updates
// into r, and renders — the canonical frame body shared by every host. It
// returns the frame for host-side presentation stats. Interactive hosts call
// it after RunTick; offline hosts call it before advancing time.
func (d *Driver) RenderFrame(elapsed time.Duration, r FrameRenderer) Frame {
	frame := d.Frame(elapsed)
	r.UpdateTextures(d.ConsumeTextureUpdates())
	r.Render(frame)
	return frame
}
