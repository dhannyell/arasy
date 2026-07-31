package ui

import (
	"github.com/dhannyell/arasy/input"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
)

const LayerBase = 10_000

// LayerOverlay opens a block of layers reserved for popups. Widgets allocate
// upward from LayerBase in declaration order, so a dropdown opened early in a
// frame would otherwise be painted over by everything declared after it. Its
// popup allocates from here instead, which is far enough above any plausible
// widget count to keep the overlay on top without the caller ordering
// anything.
const LayerOverlay = 1_000_000

type UI struct {
	state     *State
	camera    render.Camera
	reference math2d.Rect
	theme     Theme
	font      render.FontID
	frame     input.PointerFrame
	keys      input.KeyFrame
	layer     int
	overlay   int
	inside    bool
	cursorPos math2d.Vec2
	input     input.State
	activated ID
	focusable []ID
	// The repeat clock is resolved at most once per frame and shared by every
	// widget that reads a held key, so two of them do not advance it twice.
	repeatResolved bool
	repeatCode     input.SourceCode
	repeatFired    bool
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
		font:      theme.Font,
		reference: reference,
		store:     store,
		dt:        dt,
		time:      time,
		layer:     LayerBase,
		overlay:   LayerOverlay,
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
		font:      theme.Font,
		reference: reference,
		layer:     LayerBase,
		overlay:   LayerOverlay,
	}
}

func NewUpdateWithInput(state *State, frame input.PointerFrame, inputState input.State, cam render.Camera, theme Theme, reference math2d.Rect) *UI {
	u := NewUpdate(state, frame, cam, theme, reference)
	u.input = inputState
	if inputState.JustPressed(input.ActionConfirm) && state.focusID != noID {
		u.activated = state.focusID
	}
	return u
}

// NewUpdateWithKeys is NewUpdateWithInput plus the tick's keyboard frame, which
// text widgets read directly: characters and editing keys are not actions and
// no binding could describe them.
func NewUpdateWithKeys(state *State, frame input.PointerFrame, keys input.KeyFrame, inputState input.State, cam render.Camera, theme Theme, reference math2d.Rect) *UI {
	u := NewUpdateWithInput(state, frame, inputState, cam, theme, reference)
	u.keys = keys
	return u
}

// EndFrame closes the tick's UI: it cycles focus on Tab and drops it on
// Escape. Both run here rather than inside a widget because they are about the
// frame as a whole — the cycle needs every focusable already declared. Tab is
// only claimed when the frame declared at least one, so a game with no UI up
// keeps the key for itself.
func (u *UI) EndFrame() {
	if u.state == nil {
		return
	}
	if u.input.JustPressed(input.ActionCancel) {
		u.state.focusID = noID
		return
	}
	if !u.input.JustPressed(input.ActionFocusNext) || len(u.focusable) == 0 {
		return
	}

	next := u.focusable[0]
	for idx, id := range u.focusable {
		if id == u.state.focusID {
			next = u.focusable[(idx+1)%len(u.focusable)]
			break
		}
	}
	u.state.focusID = next
}

func (u *UI) Commands() []render.Command {
	return u.commands
}

func (u *UI) Emit(cmds ...render.Command) {
	u.commands = append(u.commands, cmds...)
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

// beginOverlay moves layer allocation into the reserved overlay block, so
// everything emitted until endOverlay paints above the ordinary widgets —
// including those declared after it. Swapping the two counters (rather than
// saving and restoring one) keeps both advancing, so two popups in one frame
// stack in declaration order instead of fighting for the same layer.
func (u *UI) beginOverlay() { u.layer, u.overlay = u.overlay, u.layer }

func (u *UI) endOverlay() { u.layer, u.overlay = u.overlay, u.layer }
