package ui

import (
	"github.com/dhannyell/frameforge/input"
	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
)

const LayerBase = 10_000

type UI struct {
	state     *State
	camera    render.Camera
	theme     Theme
	frame     input.PointerFrame
	layer     int
	inside    bool
	cursorPos math2d.Vec2
	commands  []render.Command
}

func New(state *State, cursorPos math2d.Vec2, inside bool, cam render.Camera, theme Theme) *UI {
	return &UI{
		state:     state,
		cursorPos: cursorPos,
		inside:    inside,
		camera:    cam,
		theme:     theme,
		layer:     LayerBase,
	}
}

// NewUpdate builds the interaction face used during Update: it carries the
// coalesced PointerFrame for the tick so Clicked/Pressed can process events.
// cursorPos/inside are seeded from the frame for hit-tests that don't depend on
// the event list.
func NewUpdate(state *State, frame input.PointerFrame, cam render.Camera, theme Theme) *UI {
	return &UI{
		state:     state,
		frame:     frame,
		cursorPos: frame.Pos,
		inside:    frame.Inside,
		camera:    cam,
		theme:     theme,
		layer:     LayerBase,
	}
}

func (u *UI) Commands() []render.Command {
	return u.commands
}

func (u *UI) screenToWorld(p math2d.Vec2) math2d.Vec2 {
	return u.camera.ScreenToWorld(p)
}

func (u *UI) screenSizeToWorld(px float64) float64 {
	return u.camera.ScreenScalarToWorld(px)
}

func (u *UI) screenRectToWorld(r math2d.Rect) math2d.Rect {
	return math2d.NewRectFromCorners(
		u.camera.ScreenToWorld(r.Min),
		u.camera.ScreenToWorld(r.Max),
	)
}

func (u *UI) nextLayer() int {
	l := u.layer
	u.layer++
	return l
}
