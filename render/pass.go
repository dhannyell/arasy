package render

import "github.com/dhannyell/arasy/math2d"

// Pass is one camera observing one command stream. Extra passes are drawn in
// declaration order before the frame's base stream.
type Pass struct {
	Camera   Camera
	Commands []Command
}

// ResolveViewport replaces a zero viewport with bounds. A pass uses the
// complete target when it does not declare an explicit viewport.
func (p Pass) ResolveViewport(bounds math2d.Rect) Pass {
	if p.Camera.Viewport == (math2d.Rect{}) {
		p.Camera.Viewport = bounds
	}
	return p
}
