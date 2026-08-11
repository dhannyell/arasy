package render

import "github.com/dhannyell/arasy/math2d"

// Pass is one camera observing one command stream. Extra passes are drawn in
// declaration order before the frame's base stream.
type Pass struct {
	Camera   Camera
	Commands []Command
	// Target names the dynamic texture this pass clears and draws into. Zero
	// draws on the screen, as before. Later passes and the base stream may
	// sample the target in the same frame. A pass must not sample its own
	// target: GPU feedback is undefined. Use two dynamic textures and alternate
	// them when a visual needs feedback. Target contents are render-only and
	// cannot be read back into the game. The first pass naming a target owns it
	// for the frame; later owners are ignored. A same-frame UpdateTexture is
	// uploaded first and then overwritten by this pass.
	Target TextureID
}

// ResolveViewport replaces a zero viewport with bounds. A pass uses the
// complete target when it does not declare an explicit viewport.
func (p Pass) ResolveViewport(bounds math2d.Rect) Pass {
	if p.Camera.Viewport == (math2d.Rect{}) {
		p.Camera.Viewport = bounds
	}
	return p
}

// CameraForScreenPoint returns the last declared screen-pass camera whose
// viewport contains point. Target passes use image coordinates and are
// ignored. Bounds resolves zero viewports. The base camera is the fallback.
// Handle UI first because the base stream draws UI above passes.
func CameraForScreenPoint(base Camera, passes []Pass, bounds math2d.Rect, point math2d.Vec2) Camera {
	for index := len(passes) - 1; index >= 0; index-- {
		if passes[index].Target != 0 {
			continue
		}
		pass := passes[index].ResolveViewport(bounds)
		if pass.Camera.Viewport.Contains(point) {
			return pass.Camera
		}
	}
	return Pass{Camera: base}.ResolveViewport(bounds).Camera
}
