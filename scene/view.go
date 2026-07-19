package scene

import "github.com/dhannyell/frameforge/render"

// WorldView is the read face of a World, handed to Game.Render. Render may
// run zero, one or many times between fixed ticks, so any mutation there
// would make simulation frame-rate dependent; this type makes that a
// compile error instead of a comment. The zero value is not usable — the
// driver builds views via World.View().
type WorldView struct {
	w *World
}

func (w *World) View() WorldView { return WorldView{w: w} }

func (v WorldView) RenderCommands() []render.Command { return v.w.RenderCommands() }
func (v WorldView) RenderCommandsInterpolated(alpha float64) []render.Command {
	return v.w.RenderCommandsInterpolated(alpha)
}
func (v WorldView) RenderCommandsInto(dst []render.Command, alpha float64) []render.Command {
	return v.w.RenderCommandsInto(dst, alpha)
}
func (v WorldView) Transform(id EntityID) (Transform, bool)           { return v.w.Transform(id) }
func (v WorldView) TransformTween(id EntityID) (TransformTween, bool) { return v.w.TransformTween(id) }
func (v WorldView) CircleSprite(id EntityID) (CircleSprite, bool)     { return v.w.CircleSprite(id) }
func (v WorldView) CircleSpriteTween(id EntityID) (CircleSpriteTween, bool) {
	return v.w.CircleSpriteTween(id)
}
func (v WorldView) RectSprite(id EntityID) (RectSprite, bool)         { return v.w.RectSprite(id) }
func (v WorldView) ImageSprite(id EntityID) (ImageSprite, bool)       { return v.w.ImageSprite(id) }
func (v WorldView) Velocity(id EntityID) (Velocity, bool)             { return v.w.Velocity(id) }
func (v WorldView) Acceleration(id EntityID) (Acceleration, bool)     { return v.w.Acceleration(id) }
func (v WorldView) Damping(id EntityID) (Damping, bool)               { return v.w.Damping(id) }
func (v WorldView) Body(id EntityID) (Body, bool)                     { return v.w.Body(id) }
func (v WorldView) CircleCollider(id EntityID) (CircleCollider, bool) { return v.w.CircleCollider(id) }
func (v WorldView) RectCollider(id EntityID) (RectCollider, bool)     { return v.w.RectCollider(id) }
func (v WorldView) Contacts() []Contact                               { return v.w.Contacts() }
func (v WorldView) StepTick() uint64                                  { return v.w.StepTick() }
