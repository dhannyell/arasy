package render

import "github.com/dhannyell/frameforge/math2d"

// ClipCommands sets clip as the Clip rect on each command in cmds that
// doesn't already carry a clip of its own (BaseCommand.Clip's zero value
// means "no clip" — see BaseCommand). It exists for code that draws an
// overlay on top of a scrollable/clipped area (e.g. a manually-drawn badge
// or accent bar per table row) one command at a time via helpers that don't
// thread a Clip parameter through — retroactively clipping the whole batch
// is simpler than adding a Clip argument to every draw helper, and keeps a
// row that's only partially inside its scroll window from bleeding its
// content past that window's edge.
func ClipCommands(cmds []Command, clip math2d.Rect) {
	for i, cmd := range cmds {
		switch c := cmd.(type) {
		case DrawRect:
			if c.Clip == (math2d.Rect{}) {
				c.Clip = clip
				cmds[i] = c
			}
		case DrawCircle:
			if c.Clip == (math2d.Rect{}) {
				c.Clip = clip
				cmds[i] = c
			}
		case DrawPoint:
			if c.Clip == (math2d.Rect{}) {
				c.Clip = clip
				cmds[i] = c
			}
		case DrawLine:
			if c.Clip == (math2d.Rect{}) {
				c.Clip = clip
				cmds[i] = c
			}
		case DrawText:
			if c.Clip == (math2d.Rect{}) {
				c.Clip = clip
				cmds[i] = c
			}
		case DrawArc:
			if c.Clip == (math2d.Rect{}) {
				c.Clip = clip
				cmds[i] = c
			}
		case DrawPolygon:
			if c.Clip == (math2d.Rect{}) {
				c.Clip = clip
				cmds[i] = c
			}
		}
	}
}
