package runtime

import (
	"testing"

	"github.com/dhannyell/frameforge/input"
	"github.com/dhannyell/frameforge/math2d"
)

func TestDriverInputBeginTickMapsInputState(t *testing.T) {
	driverInput := newDriverInput([]input.Binding{
		{
			Source: input.Source{Device: input.DeviceKeyboard, Code: input.KeyD},
			Action: input.ActionMoveX,
			Scale:  1,
		},
	})

	driverInput.Apply([]input.Event{
		{
			Source: input.Source{Device: input.DeviceKeyboard, Code: input.KeyD},
			Kind:   input.EventButtonDown,
		},
	}, 7)

	state := driverInput.BeginTick()
	if got := state.Value(input.ActionMoveX); got != 1 {
		t.Fatalf("move x = %v, want 1", got)
	}
	if !state.JustPressed(input.ActionMoveX) {
		t.Fatal("move x should be just pressed")
	}

	events := driverInput.buffer.Events(7)
	if len(events) != 1 {
		t.Fatalf("buffer events = %d, want 1", len(events))
	}
	if events[0].Tick != 7 {
		t.Fatalf("buffer event tick = %d, want 7", events[0].Tick)
	}
}

func TestDriverInputPointerFrameSeedsFromPersistentPointerAfterEndTick(t *testing.T) {
	driverInput := newDriverInput(nil)

	driverInput.Apply([]input.Event{
		mouseButtonEvent(input.EventButtonDown, 15, 20),
	}, 0)

	frame := driverInput.PointerFrame()
	if frame.Pos != (math2d.Vec2{X: 15, Y: 20}) || !frame.Inside || !frame.Buttons[input.PointerPrimary] {
		t.Fatalf("frame after down = %+v, want pos 15,20 inside primary", frame)
	}
	if len(frame.Events) != 1 {
		t.Fatalf("frame events after down = %d, want 1", len(frame.Events))
	}

	driverInput.EndTick(1)

	frame = driverInput.PointerFrame()
	if frame.Pos != (math2d.Vec2{X: 15, Y: 20}) || !frame.Inside || !frame.Buttons[input.PointerPrimary] {
		t.Fatalf("frame after EndTick = %+v, want persistent pos/inside/buttons", frame)
	}
	if len(frame.Events) != 0 {
		t.Fatalf("frame events after EndTick = %d, want 0", len(frame.Events))
	}
}

func TestDriverInputPointerLeaveReleasesPressedButton(t *testing.T) {
	driverInput := newDriverInput(nil)

	driverInput.Apply([]input.Event{
		mouseButtonEvent(input.EventButtonDown, 15, 20),
	}, 0)
	driverInput.EndTick(1)

	driverInput.Apply([]input.Event{
		{
			Source: input.Source{Device: input.DeviceMouse, Code: input.MouseUnknown},
			Kind:   input.EventPointerLeave,
		},
	}, 1)

	frame := driverInput.PointerFrame()
	if frame.Inside {
		t.Fatal("frame should be outside after pointer leave")
	}
	if frame.Buttons[input.PointerPrimary] {
		t.Fatal("primary button should be released in frame after pointer leave")
	}
	if len(frame.Events) != 1 {
		t.Fatalf("frame events after pointer leave = %d, want 1 synthetic release", len(frame.Events))
	}
	event := frame.Events[0]
	if event.Kind != input.EventButtonUp || event.Button != input.PointerPrimary {
		t.Fatalf("synthetic event = %+v, want primary button up", event)
	}
	pointer := driverInput.Pointer()
	if pointer.Button(input.PointerPrimary) {
		t.Fatal("persistent pointer primary should be released")
	}
	if pointer.Inside() {
		t.Fatal("persistent pointer should be outside")
	}
}

func TestDriverInputPointerMoveDoesNotAffectMappedActions(t *testing.T) {
	driverInput := newDriverInput([]input.Binding{
		{
			Source: input.Source{Device: input.DeviceMouse, Code: input.MouseX},
			Action: input.ActionMoveX,
			Scale:  1,
		},
	})

	driverInput.Apply([]input.Event{
		{
			Source: input.Source{Device: input.DeviceMouse, Code: input.MouseX},
			Kind:   input.EventPointerMove,
			X:      40,
			Y:      50,
		},
	}, 0)

	state := driverInput.BeginTick()
	if got := state.Value(input.ActionMoveX); got != 0 {
		t.Fatalf("move x from pointer move = %v, want 0", got)
	}
	pointer := driverInput.Pointer()
	if pointer.Pos() != (math2d.Vec2{X: 40, Y: 50}) {
		t.Fatalf("pointer pos = %+v, want 40,50", pointer.Pos())
	}
}
