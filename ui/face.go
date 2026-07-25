package ui

import (
	"github.com/dhannyell/arasy/ease"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
)

// drawFace adapts *UI to the Drawer contract for the Render phase.
type drawFace struct{ u *UI }

func (f drawFace) In(space Space) DrawScope { return DrawScope{u: f.u, space: space} }

func (f drawFace) RowHeight() float64                         { return f.u.RowHeight() }
func (f drawFace) HeaderHeight() float64                      { return f.u.HeaderHeight() }
func (f drawFace) TableBodyRect(rect math2d.Rect) math2d.Rect { return f.u.TableBodyRect(rect) }
func (f drawFace) SetFont(id render.FontID)                   { f.u.SetFont(id) }

func (f drawFace) Approach(id string, target, speed float64) float64 {
	return f.u.Approach(id, target, speed)
}
func (f drawFace) Spring(id string, target float64, cfg SpringConfig) float64 {
	return f.u.Spring(id, target, cfg)
}
func (f drawFace) SpringScale(id string, target float64, cfg SpringConfig, rect math2d.Rect) math2d.Rect {
	return f.u.SpringScale(id, target, cfg, rect)
}
func (f drawFace) Tween(id string, dur float64, k ease.Kind) float64 { return f.u.Tween(id, dur, k) }
func (f drawFace) Clock(id string) float64                           { return f.u.Clock(id) }
func (f drawFace) Reset(id string)                                   { f.u.Reset(id) }
func (f drawFace) SequenceKeyed(id string, key uint64) Sequence      { return f.u.SequenceKeyed(id, key) }
func (f drawFace) Effect(id string, opts EffectOptions) Effect       { return f.u.Effect(id, opts) }

func (f drawFace) Emit(cmds ...render.Command)  { f.u.Emit(cmds...) }
func (f drawFace) BeginGroup(opts GroupOptions) { f.u.BeginGroup(opts) }
func (f drawFace) EndGroup()                    { f.u.EndGroup() }

func (f drawFace) ScreenTransition(id string, key uint64, opts TransitionOptions) ScreenTransition {
	return f.u.ScreenTransition(id, key, opts)
}

// interactFace adapts *UI to the Interactor contract for the Update phase.
type interactFace struct{ u *UI }

func (f interactFace) In(space Space) InteractScope { return InteractScope{u: f.u, space: space} }

func (f interactFace) RowHeight() float64                         { return f.u.RowHeight() }
func (f interactFace) HeaderHeight() float64                      { return f.u.HeaderHeight() }
func (f interactFace) TableBodyRect(rect math2d.Rect) math2d.Rect { return f.u.TableBodyRect(rect) }

// Draw returns the Render-phase face of this UI.
func (u *UI) Draw() Drawer { return drawFace{u} }

// Interact returns the Update-phase face of this UI.
func (u *UI) Interact() Interactor { return interactFace{u} }
