package runtime

import (
	"time"

	"github.com/dhannyell/arasy/render"
)

type Frame struct {
	Camera   render.Camera
	Commands []render.Command
}

// FrameUpdates carries the render-only resource publications drained from
// the driver before one frame: dynamic textures and retained sprite/text
// batches. Renderers must apply textures first — retained batches may
// reference them.
type FrameUpdates struct {
	Textures      []TextureUpdate
	SpriteBatches []SpriteBatchUpdate
	TextBatches   []TextBatchUpdate
}

// FrameRenderer is what a host renderer must implement to receive one frame:
// first the resource updates drained from the driver, then the frame itself.
// Both wasm renderers and the raster target satisfy it.
type FrameRenderer interface {
	UpdateResources(updates FrameUpdates)
	Render(frame Frame)
}

// RenderFrame assembles the frame, drains pending resource updates into r,
// and renders — the canonical frame body shared by every host. It returns
// the frame for host-side presentation stats. Interactive hosts call it
// after RunTick; offline hosts call it before advancing time.
func (d *Driver) RenderFrame(elapsed time.Duration, r FrameRenderer) Frame {
	frame := d.Frame(elapsed)
	r.UpdateResources(d.ConsumeFrameUpdates())
	r.Render(frame)
	return frame
}
