package ui

import (
	"testing"

	"github.com/dhannyell/frameforge/input"
	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
)

func sliderRect() math2d.Rect {
	return math2d.NewRect(math2d.Vec2{X: 100, Y: 100}, math2d.Vec2{X: 200, Y: 40})
}

func sliderOptions() SliderOptions {
	return SliderOptions{Min: 0, Max: 1}
}

func sliderPointerFrame(pos math2d.Vec2, primaryDown bool, events ...input.PointerEvent) input.PointerFrame {
	frame := input.PointerFrame{Pos: pos, Inside: true, Events: events}
	frame.Buttons[input.PointerPrimary] = primaryDown
	return frame
}

func TestSliderInputPressCapturesAndJumpsToPointer(t *testing.T) {
	state := &State{}
	rect := sliderRect()
	pos := rect.Center()
	u := NewUpdate(state, sliderPointerFrame(pos, true,
		input.PointerEvent{Kind: input.EventButtonDown, Button: input.PointerPrimary, Pos: pos},
	), testInteractCamera(), DefaultTheme(), testInteractCamera().Viewport)

	value, changed := u.sliderInputIn("density", rect, 0, sliderOptions(), SpaceRef)

	if !changed || value != 0.5 {
		t.Fatalf("SliderInput() = (%v, %v), want (0.5, true)", value, changed)
	}
	if state.activeID != HashID("density") {
		t.Fatalf("activeID = %v, want density slider", state.activeID)
	}
	if state.focusID != HashID("density") {
		t.Fatalf("focusID = %v, want density slider", state.focusID)
	}
}

func TestSliderInputDragUsesCoalescedPositionAndClampsOutside(t *testing.T) {
	rect := sliderRect()
	state := &State{activeID: HashID("density")}
	pos := math2d.Vec2{X: rect.Max.X + 100, Y: rect.Center().Y}
	u := NewUpdate(state, sliderPointerFrame(pos, true), testInteractCamera(), DefaultTheme(), testInteractCamera().Viewport)

	value, changed := u.sliderInputIn("density", rect, 0.25, sliderOptions(), SpaceRef)

	if !changed || value != 1 {
		t.Fatalf("SliderInput() = (%v, %v), want (1, true)", value, changed)
	}
	if state.activeID != HashID("density") {
		t.Fatalf("activeID = %v, want capture retained while primary is down", state.activeID)
	}
}

func TestSliderInputReleaseOutsideUpdatesAndClearsCapture(t *testing.T) {
	rect := sliderRect()
	state := &State{activeID: HashID("density")}
	pos := math2d.Vec2{X: rect.Min.X - 100, Y: rect.Center().Y}
	u := NewUpdate(state, sliderPointerFrame(pos, false,
		input.PointerEvent{Kind: input.EventButtonUp, Button: input.PointerPrimary, Pos: pos},
	), testInteractCamera(), DefaultTheme(), testInteractCamera().Viewport)

	value, changed := u.sliderInputIn("density", rect, 0.75, sliderOptions(), SpaceRef)

	if !changed || value != 0 {
		t.Fatalf("SliderInput() = (%v, %v), want (0, true)", value, changed)
	}
	if state.activeID != noID {
		t.Fatalf("activeID = %v, want released", state.activeID)
	}
}

func TestSliderInputSameTickPressAndReleaseUsesReleasePosition(t *testing.T) {
	rect := sliderRect()
	theme := DefaultTheme()
	track, _, ok := sliderGeometry(rect, theme.Slider)
	if !ok {
		t.Fatal("expected valid slider geometry")
	}
	down := math2d.Vec2{X: track.Min.X, Y: rect.Center().Y}
	up := math2d.Vec2{X: track.Max.X, Y: rect.Center().Y}
	state := &State{}
	u := NewUpdate(state, sliderPointerFrame(up, false,
		input.PointerEvent{Kind: input.EventButtonDown, Button: input.PointerPrimary, Pos: down},
		input.PointerEvent{Kind: input.EventButtonUp, Button: input.PointerPrimary, Pos: up},
	), testInteractCamera(), theme, testInteractCamera().Viewport)

	value, changed := u.sliderInputIn("density", rect, 0.5, sliderOptions(), SpaceRef)

	if !changed || value != 1 {
		t.Fatalf("SliderInput() = (%v, %v), want release value (1, true)", value, changed)
	}
	if state.activeID != noID {
		t.Fatalf("activeID = %v, want released in same tick", state.activeID)
	}
}

func TestSliderInputSnapsToStep(t *testing.T) {
	rect := sliderRect()
	theme := DefaultTheme()
	track, _, _ := sliderGeometry(rect, theme.Slider)
	pos := math2d.Vec2{X: track.Min.X + track.Width()*0.62, Y: rect.Center().Y}
	u := NewUpdate(&State{}, sliderPointerFrame(pos, true,
		input.PointerEvent{Kind: input.EventButtonDown, Button: input.PointerPrimary, Pos: pos},
	), testInteractCamera(), theme, testInteractCamera().Viewport)

	value, changed := u.sliderInputIn("density", rect, 0, SliderOptions{Min: 0, Max: 1, Step: 0.25}, SpaceRef)

	if !changed || value != 0.5 {
		t.Fatalf("SliderInput() = (%v, %v), want snapped (0.5, true)", value, changed)
	}
}

func TestSliderInputCanReachMaxWhenStepDoesNotDivideRange(t *testing.T) {
	rect := sliderRect()
	theme := DefaultTheme()
	track, _, _ := sliderGeometry(rect, theme.Slider)
	pos := math2d.Vec2{X: track.Max.X, Y: rect.Center().Y}
	u := NewUpdate(&State{}, sliderPointerFrame(pos, true,
		input.PointerEvent{Kind: input.EventButtonDown, Button: input.PointerPrimary, Pos: pos},
	), testInteractCamera(), theme, testInteractCamera().Viewport)

	value, changed := u.sliderInputIn("density", rect, 0, SliderOptions{Min: 0, Max: 1, Step: 0.3}, SpaceRef)

	if !changed || value != 1 {
		t.Fatalf("SliderInput() = (%v, %v), want max endpoint (1, true)", value, changed)
	}
}

func TestSliderInputNormalizesExternalValueWithoutPointerInput(t *testing.T) {
	u := NewUpdate(&State{}, sliderPointerFrame(math2d.Vec2{}, false), testInteractCamera(), DefaultTheme(), testInteractCamera().Viewport)

	value, changed := u.sliderInputIn("density", sliderRect(), 2, sliderOptions(), SpaceRef)

	if !changed || value != 1 {
		t.Fatalf("SliderInput() = (%v, %v), want clamped (1, true)", value, changed)
	}
}

func TestSliderInputDoesNotStealCaptureFromAnotherWidget(t *testing.T) {
	rect := sliderRect()
	pos := rect.Center()
	other := HashID("other")
	state := &State{activeID: other}
	u := NewUpdate(state, sliderPointerFrame(pos, true,
		input.PointerEvent{Kind: input.EventButtonDown, Button: input.PointerPrimary, Pos: pos},
	), testInteractCamera(), DefaultTheme(), testInteractCamera().Viewport)

	value, changed := u.sliderInputIn("density", rect, 0.25, sliderOptions(), SpaceRef)

	if changed || value != 0.25 {
		t.Fatalf("SliderInput() = (%v, %v), want unchanged (0.25, false)", value, changed)
	}
	if state.activeID != other {
		t.Fatalf("activeID = %v, want other widget capture %v", state.activeID, other)
	}
}

func TestSliderInputIgnoresSecondaryButton(t *testing.T) {
	rect := sliderRect()
	pos := rect.Center()
	state := &State{}
	u := NewUpdate(state, sliderPointerFrame(pos, false,
		input.PointerEvent{Kind: input.EventButtonDown, Button: input.PointerSecondary, Pos: pos},
	), testInteractCamera(), DefaultTheme(), testInteractCamera().Viewport)

	value, changed := u.sliderInputIn("density", rect, 0.25, sliderOptions(), SpaceRef)

	if changed || value != 0.25 || state.activeID != noID {
		t.Fatalf("secondary press produced value=%v changed=%v activeID=%v", value, changed, state.activeID)
	}
}

func TestSliderDrawsTrackFillAndThumbAtValue(t *testing.T) {
	cam := testCamera()
	store := AnimStore{}
	u := New(&State{}, math2d.Vec2{}, true, cam, DefaultTheme(), cam.Viewport, &store, 0, 0)
	rect := sliderRect()

	u.sliderIn("density", rect, 0.5, sliderOptions(), SpaceRef)

	commands := u.Commands()
	if len(commands) != 3 {
		t.Fatalf("len(commands) = %d, want track, fill and thumb", len(commands))
	}
	track, ok := commands[0].(render.DrawRect)
	if !ok {
		t.Fatalf("track type = %T, want render.DrawRect", commands[0])
	}
	fill, ok := commands[1].(render.DrawRect)
	if !ok {
		t.Fatalf("fill type = %T, want render.DrawRect", commands[1])
	}
	thumb, ok := commands[2].(render.DrawCircle)
	if !ok {
		t.Fatalf("thumb type = %T, want render.DrawCircle", commands[2])
	}
	if fill.Rect.Width() != track.Rect.Width()/2 {
		t.Fatalf("fill width = %v, want half track width %v", fill.Rect.Width(), track.Rect.Width()/2)
	}
	if thumb.Center.X != rect.Center().X || thumb.Center.Y != rect.Center().Y {
		t.Fatalf("thumb center = %v, want rect center %v", thumb.Center, rect.Center())
	}
}

func TestSliderDrawUsesActiveThumbAndFocusRing(t *testing.T) {
	cam := testCamera()
	store := AnimStore{}
	state := &State{activeID: HashID("density"), focusID: HashID("density")}
	theme := DefaultTheme()
	u := New(state, math2d.Vec2{}, true, cam, theme, cam.Viewport, &store, 0, 0)

	u.sliderIn("density", sliderRect(), 0.5, sliderOptions(), SpaceRef)

	commands := u.Commands()
	if len(commands) != 4 {
		t.Fatalf("len(commands) = %d, want track, fill, thumb and focus ring", len(commands))
	}
	thumb := commands[2].(render.DrawCircle)
	if thumb.Fill != theme.Slider.ThumbActive {
		t.Fatalf("thumb fill = %v, want active color %v", thumb.Fill, theme.Slider.ThumbActive)
	}
	if _, ok := commands[3].(render.DrawRect); !ok {
		t.Fatalf("focus command type = %T, want render.DrawRect", commands[3])
	}
}

func TestSliderInUsesSameScreenSpaceForInputAndDrawing(t *testing.T) {
	cam := render.Camera{
		World:    math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 100, Y: 100}),
		Viewport: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 400, Y: 400}),
		Aspect:   render.AspectStretch,
	}
	rect := math2d.NewRect(math2d.Vec2{X: 100, Y: 180}, math2d.Vec2{X: 200, Y: 40})
	pos := rect.Center()
	state := &State{}
	update := NewUpdate(state, sliderPointerFrame(pos, true,
		input.PointerEvent{Kind: input.EventButtonDown, Button: input.PointerPrimary, Pos: pos},
	), cam, DefaultTheme(), cam.Viewport)

	value, changed := update.sliderInputIn("density", rect, 0, sliderOptions(), SpaceScreen)
	if !changed || value != 0.5 {
		t.Fatalf("sliderInputIn() = (%v, %v), want screen-space (0.5, true)", value, changed)
	}

	store := AnimStore{}
	draw := New(state, pos, true, cam, DefaultTheme(), cam.Viewport, &store, 0, 0)
	draw.sliderIn("density", rect, value, sliderOptions(), SpaceScreen)
	thumb := draw.Commands()[2].(render.DrawCircle)
	wantWorld := cam.ScreenToWorld(pos)
	if thumb.Center != wantWorld {
		t.Fatalf("thumb center = %v, want screen position mapped to world %v", thumb.Center, wantWorld)
	}
}

func TestSliderInvalidRangeFailsClosedAtMin(t *testing.T) {
	u := NewUpdate(&State{}, sliderPointerFrame(math2d.Vec2{}, false), testInteractCamera(), DefaultTheme(), testInteractCamera().Viewport)

	value, changed := u.sliderInputIn("density", sliderRect(), 8, SliderOptions{Min: 3, Max: 3}, SpaceRef)

	if !changed || value != 3 {
		t.Fatalf("SliderInput() = (%v, %v), want disabled range to normalize to min", value, changed)
	}
}
