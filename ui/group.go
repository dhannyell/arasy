package ui

import (
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
)

// GroupOptions transforms every command emitted between BeginGroup and
// EndGroup. The zero value is a no-op group: no offset, fully opaque.
type GroupOptions struct {
	// Offset displaces all commands, expressed in Space coordinates.
	Offset math2d.Vec2
	// Fade multiplies command alphas down: 0 = opaque, 1 = invisible.
	// Values outside [0,1] are clamped.
	Fade float64
	// Space interprets Offset (default SpaceRef).
	Space Space
}

type groupFrame struct {
	start int
	opts  GroupOptions
}

// BeginGroup opens a command group. Groups nest: inner groups are applied
// first, outer groups transform the already-transformed result.
func (u *UI) BeginGroup(opts GroupOptions) {
	u.groups = append(u.groups, groupFrame{start: len(u.commands), opts: opts})
}

// EndGroup closes the innermost open group and applies it's offset and fade
// to the commands emitted since the matching BeginGroup. Calling it with no
// open group is a no-op.
func (u *UI) EndGroup() {
	if len(u.groups) == 0 {
		return
	}
	frame := u.groups[len(u.groups)-1]
	u.groups = u.groups[:len(u.groups)-1]

	alpha := math2d.Clamp(1-frame.opts.Fade, 0, 1)
	offset := u.offsetToWorld(frame.opts.Offset, frame.opts.Space)
	if offset == (math2d.Vec2{}) && alpha == 1 {
		return
	}
	for i := frame.start; i < len(u.commands); i++ {
		u.commands[i] = transformCommand(u.commands[i], offset, alpha)
	}
}

// offsetToWorld converts a displacement vector from the given
// UI space to world coordinates.
func (u *UI) offsetToWorld(offset math2d.Vec2, space Space) math2d.Vec2 {
	origin := u.toWorld(math2d.Vec2{}, space)
	moved := u.toWorld(offset, space)
	return moved.Sub(origin)
}

// transformCommand returns cmd displaced by offset with its colors' alpha
// multiplied. Unknown command types pass through untouched.
func transformCommand(cmd render.Command, offset math2d.Vec2, alpha float64) render.Command {
	switch c := cmd.(type) {
	case render.DrawRect:
		c.Rect = math2d.NewRect(c.Rect.Min.Add(offset), c.Rect.Size())
		c.Fill = multiplyAlpha(c.Fill, alpha)
		c.Stroke = multiplyAlpha(c.Stroke, alpha)
		return c
	case render.DrawCircle:
		c.Center = c.Center.Add(offset)
		c.Fill = multiplyAlpha(c.Fill, alpha)
		c.Stroke = multiplyAlpha(c.Stroke, alpha)
		return c
	case render.DrawPoint:
		c.Center = c.Center.Add(offset)
		c.Color = multiplyAlpha(c.Color, alpha)
		return c
	case render.DrawLine:
		c.From = c.From.Add(offset)
		c.To = c.To.Add(offset)
		c.Color = multiplyAlpha(c.Color, alpha)
		return c
	case render.DrawText:
		c.Position = c.Position.Add(offset)
		c.Color = multiplyAlpha(c.Color, alpha)
		return c
	case render.DrawArc:
		c.Center = c.Center.Add(offset)
		c.Color = multiplyAlpha(c.Color, alpha)
		return c
	default:
		return cmd
	}
}
