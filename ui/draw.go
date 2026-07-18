package ui

import (
	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
)

// Draw emits raw, theme-independent shapes stacked in call order (each call
// takes the next layer via nextLayer()). Unlike Panel/Button/Label, these
// carry explicit colors and sizes instead of pulling from the theme — use
// them for game-specific chrome (HUD panels, badges, motifs) that doesn't
// belong to a themed widget.

// fillRectIn draws a solid rectangle with no stroke.
func (u *UI) fillRectIn(rect math2d.Rect, fill render.Color, space Space) {
	u.commands = append(u.commands, render.DrawRect{
		BaseCommand: render.BaseCommand{LayerValue: u.nextLayer()},
		Rect:        u.rectToWorld(rect, space),
		Fill:        fill,
	})
}

// drawTextIn draws unrotated text at pos with an explicit color/size.
func (u *UI) drawTextIn(text string, pos math2d.Vec2, color render.Color, size float64, align render.TextAlign, baseline render.TextBaseline, space Space) {
	u.drawTextRotIn(text, pos, color, size, 0, align, baseline, space)
}

// drawTextRotIn draws text rotated by rotation radians around pos.
func (u *UI) drawTextRotIn(text string, pos math2d.Vec2, color render.Color, size, rotation float64, align render.TextAlign, baseline render.TextBaseline, space Space) {
	u.commands = append(u.commands, render.DrawText{
		BaseCommand: render.BaseCommand{LayerValue: u.nextLayer()},
		Position:    u.toWorld(pos, space),
		Text:        text,
		Color:       color,
		Size:        u.sizeToWorld(size, space),
		Font:        u.font,
		Align:       align,
		Baseline:    baseline,
		Rotation:    rotation,
	})
}

// SetFont selects the font used by the raw DrawText primitives from this call
// onward, without adding a font argument to every draw signature (ImGui-style
// current-font state). It starts at the theme's Font each frame; pass zero to
// go back to the built-in engine font. Themed widgets (Label, Button, Badge,
// ...) always use Theme.Font and ignore this override.
func (u *UI) SetFont(id render.FontID) {
	u.font = id
}

// drawCircleIn draws a circle with optional fill and/or stroke.
func (u *UI) drawCircleIn(center math2d.Vec2, radius float64, fill, stroke render.Color, strokeWidth float64, space Space) {
	u.commands = append(u.commands, render.DrawCircle{
		BaseCommand: render.BaseCommand{LayerValue: u.nextLayer()},
		Center:      u.toWorld(center, space),
		Radius:      u.sizeToWorld(radius, space),
		Fill:        fill,
		Stroke:      stroke,
		StrokeWidth: u.sizeToWorld(strokeWidth, space),
	})
}

// drawLineIn draws a straight line segment.
func (u *UI) drawLineIn(from, to math2d.Vec2, width float64, color render.Color, space Space) {
	u.commands = append(u.commands, render.DrawLine{
		BaseCommand: render.BaseCommand{LayerValue: u.nextLayer()},
		From:        u.toWorld(from, space),
		To:          u.toWorld(to, space),
		Width:       u.sizeToWorld(width, space),
		Color:       color,
	})
}

// drawPolyIn draws a filled and/or stroked polygon from points.
func (u *UI) drawPolyIn(fill, stroke render.Color, strokeWidth float64, space Space, points ...math2d.Vec2) {
	worldPoints := make([]math2d.Vec2, len(points))
	for i, p := range points {
		worldPoints[i] = u.toWorld(p, space)
	}
	u.commands = append(u.commands, render.DrawPolygon{
		BaseCommand: render.BaseCommand{LayerValue: u.nextLayer()},
		Points:      worldPoints,
		Fill:        fill,
		Stroke:      stroke,
		StrokeWidth: u.sizeToWorld(strokeWidth, space),
	})
}
