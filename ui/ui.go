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
	reference math2d.Rect
	theme     Theme
	frame     input.PointerFrame
	layer     int
	inside    bool
	cursorPos math2d.Vec2
	commands  []render.Command
	store     *AnimStore
	groups    []groupFrame
	dt        float64
	time      float64
}

// New constructs the render face used by the runtime driver each Render/Init.
// Games receive UI values through context and should not build them directly.
func New(state *State, cursorPos math2d.Vec2, inside bool, cam render.Camera, theme Theme, reference math2d.Rect, store *AnimStore, dt, time float64) *UI {
	return &UI{
		state:     state,
		cursorPos: cursorPos,
		inside:    inside,
		camera:    cam,
		theme:     theme,
		reference: reference,
		store:     store,
		dt:        dt,
		time:      time,
		layer:     LayerBase,
	}
}

// NewUpdate constructs the interaction face used by the runtime driver each
// Update. Games receive UI values through context and should not build them
// directly. It carries the coalesced PointerFrame for the tick so
// Clicked/Pressed can process events.
func NewUpdate(state *State, frame input.PointerFrame, cam render.Camera, theme Theme, reference math2d.Rect) *UI {
	return &UI{
		state:     state,
		frame:     frame,
		cursorPos: frame.Pos,
		inside:    frame.Inside,
		camera:    cam,
		theme:     theme,
		reference: reference,
		layer:     LayerBase,
	}
}

func (u *UI) Commands() []render.Command {
	return u.commands
}

func (u *UI) refCamera() render.Camera {
	return render.Camera{
		World:    u.reference,
		Viewport: u.camera.Viewport,
		Aspect:   render.AspectFit,
	}
}

func (u *UI) playfieldRect() math2d.Rect {
	return math2d.NewRectFromCorners(
		u.camera.WorldToScreen(u.camera.World.Min),
		u.camera.WorldToScreen(u.camera.World.Max))
}

func (u *UI) playfieldCamera() render.Camera {
	return render.Camera{World: u.reference, Viewport: u.playfieldRect(), Aspect: render.AspectFit}
}

func (u *UI) cameraFor(space Space) render.Camera {
	switch space {
	case SpacePlayfield:
		return u.playfieldCamera()
	case SpaceScreen:
		return render.Camera{World: u.camera.Viewport, Viewport: u.camera.Viewport} // identidade
	default:
		return u.refCamera()
	}
}

func (u *UI) toWorld(p math2d.Vec2, space Space) math2d.Vec2 {
	return u.camera.ScreenToWorld(u.cameraFor(space).WorldToScreen(p))
}

func (u *UI) sizeToWorld(v float64, space Space) float64 {
	return u.camera.ScreenScalarToWorld(u.cameraFor(space).WorldScalarToScreen(v))
}

func (u *UI) rectToWorld(r math2d.Rect, space Space) math2d.Rect {
	return math2d.NewRectFromCorners(u.toWorld(r.Min, space), u.toWorld(r.Max, space))
}

func (u *UI) cursorIn(space Space) math2d.Vec2 {
	return u.cameraFor(space).ScreenToWorld(u.cursorPos)
}

func (u *UI) nextLayer() int {
	l := u.layer
	u.layer++
	return l
}
