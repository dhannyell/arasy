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

// CameraForScreenPoint returns the last declared extra-pass camera whose
// viewport contains point. Bounds resolves zero viewports. The base camera is
// the fallback. Handle UI first because the base stream draws UI above passes.
func CameraForScreenPoint(base Camera, passes []Pass, bounds math2d.Rect, point math2d.Vec2) Camera {
	for index := len(passes) - 1; index >= 0; index-- {
		pass := passes[index].ResolveViewport(bounds)
		if pass.Camera.Viewport.Contains(point) {
			return pass.Camera
		}
	}
	return Pass{Camera: base}.ResolveViewport(bounds).Camera
}
