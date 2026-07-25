package input_test

import (
	"testing"

	"github.com/dhannyell/arasy/input"
)

// These tests cover the sticky-press latch: hosts collect OS/browser input
// events during a render frame and apply them in one batch stamped with the
// current tick. Without latching, a press+release landing entirely inside
// one batch collapses to "never pressed" and the tap is silently dropped.

func TestMapperLatchesSubTickTapAsPressedForOneTick(t *testing.T) {
	mapper := input.NewMapper([]input.Binding{
		{Source: keyboard(input.KeySpace), Action: input.ActionConfirm, Scale: 1},
	})

	mapper.Apply(input.Event{Source: keyboard(input.KeySpace), Kind: input.EventButtonDown})
	mapper.Apply(input.Event{Source: keyboard(input.KeySpace), Kind: input.EventButtonUp})

	state := mapper.State()
	if !state.Pressed(input.ActionConfirm) {
		t.Fatal("expected confirm to be pressed on the tap tick")
	}
	if !state.JustPressed(input.ActionConfirm) {
		t.Fatal("expected confirm to be just pressed on the tap tick")
	}

	mapper.EndFrame()

	state = mapper.State()
	if state.Pressed(input.ActionConfirm) {
		t.Fatal("expected confirm to not be pressed the tick after the tap")
	}
	if !state.JustReleased(input.ActionConfirm) {
		t.Fatal("expected confirm to be just released the tick after the tap")
	}
}

func TestMapperKeepsActionPressedWhenKeyIsHeldAfterTap(t *testing.T) {
	mapper := input.NewMapper([]input.Binding{
		{Source: keyboard(input.KeySpace), Action: input.ActionConfirm, Scale: 1},
	})

	// down + up + down within the same batch: the key is genuinely held
	// going into the next tick, not just latched from a sub-tick tap.
	mapper.Apply(input.Event{Source: keyboard(input.KeySpace), Kind: input.EventButtonDown})
	mapper.Apply(input.Event{Source: keyboard(input.KeySpace), Kind: input.EventButtonUp})
	mapper.Apply(input.Event{Source: keyboard(input.KeySpace), Kind: input.EventButtonDown})

	state := mapper.State()
	if !state.Pressed(input.ActionConfirm) {
		t.Fatal("expected confirm to be pressed on the tap tick")
	}

	mapper.EndFrame()

	state = mapper.State()
	if !state.Pressed(input.ActionConfirm) {
		t.Fatal("expected confirm to still be pressed after EndFrame (key genuinely held)")
	}
	if state.JustReleased(input.ActionConfirm) {
		t.Fatal("did not expect a just-released edge while the key is still held")
	}
}

func TestMapperTapOnOneSourceDoesNotEdgeActionHeldByAnotherSource(t *testing.T) {
	mapper := input.NewMapper([]input.Binding{
		{Source: keyboard(input.KeySpace), Action: input.ActionConfirm, Scale: 1},
		{Source: keyboard(input.KeyEnter), Action: input.ActionConfirm, Scale: 1},
	})

	// KeySpace is already held from a previous tick.
	mapper.Apply(input.Event{Source: keyboard(input.KeySpace), Kind: input.EventButtonDown})
	_ = mapper.State()
	mapper.EndFrame()

	// KeyEnter taps within the same batch while KeySpace stays held.
	mapper.Apply(input.Event{Source: keyboard(input.KeyEnter), Kind: input.EventButtonDown})
	mapper.Apply(input.Event{Source: keyboard(input.KeyEnter), Kind: input.EventButtonUp})

	state := mapper.State()
	if !state.Pressed(input.ActionConfirm) {
		t.Fatal("expected confirm to remain pressed")
	}
	if state.JustPressed(input.ActionConfirm) {
		t.Fatal("did not expect just pressed: action was already pressed via the held key")
	}
	if state.JustReleased(input.ActionConfirm) {
		t.Fatal("did not expect just released: action is still pressed via the held key")
	}

	mapper.EndFrame()

	state = mapper.State()
	if !state.Pressed(input.ActionConfirm) {
		t.Fatal("expected confirm to remain pressed after EndFrame (KeySpace still held)")
	}
	if state.JustReleased(input.ActionConfirm) {
		t.Fatal("did not expect just released: KeySpace was never released")
	}
}

func TestMapperDoesNotLatchAxisSources(t *testing.T) {
	mapper := input.NewMapper([]input.Binding{
		{Source: gamepad(input.GamepadLeftStickX), Action: input.ActionMoveX, Scale: 1},
	})

	mapper.Apply(input.Event{Source: gamepad(input.GamepadLeftStickX), Kind: input.EventAxisChanged, Value: 1})
	mapper.Apply(input.Event{Source: gamepad(input.GamepadLeftStickX), Kind: input.EventAxisChanged, Value: 0})

	state := mapper.State()
	if got := state.Value(input.ActionMoveX); got != 0 {
		t.Fatalf("move x = %v, want 0 (axis sources must not latch)", got)
	}
	if state.Pressed(input.ActionMoveX) {
		t.Fatal("did not expect axis source to be reported as pressed")
	}
}

func TestMapperCloneAndResetHandleLatch(t *testing.T) {
	mapper := input.NewMapper([]input.Binding{
		{Source: keyboard(input.KeySpace), Action: input.ActionConfirm, Scale: 1},
	})

	mapper.Apply(input.Event{Source: keyboard(input.KeySpace), Kind: input.EventButtonDown})
	mapper.Apply(input.Event{Source: keyboard(input.KeySpace), Kind: input.EventButtonUp})

	clone := mapper.Clone()

	original := mapper.State()
	cloned := clone.State()

	if original.Pressed(input.ActionConfirm) != cloned.Pressed(input.ActionConfirm) {
		t.Fatal("expected clone pressed state to match original")
	}
	if original.JustPressed(input.ActionConfirm) != cloned.JustPressed(input.ActionConfirm) {
		t.Fatal("expected clone just-pressed state to match original")
	}
	if !cloned.Pressed(input.ActionConfirm) {
		t.Fatal("expected clone to preserve the latch from the source mapper")
	}

	mapper.Reset()

	state := mapper.State()
	if state.Pressed(input.ActionConfirm) {
		t.Fatal("expected Reset to clear the latch")
	}
}

func TestMapperKeyUpOnNeverPressedKeyDoesNotCreatePhantomPress(t *testing.T) {
	mapper := input.NewMapper([]input.Binding{
		{Source: keyboard(input.KeySpace), Action: input.ActionConfirm, Scale: 1},
	})

	mapper.Apply(input.Event{Source: keyboard(input.KeySpace), Kind: input.EventButtonUp})

	state := mapper.State()
	if state.Pressed(input.ActionConfirm) {
		t.Fatal("did not expect a phantom press from a stray button-up event")
	}
	if state.JustPressed(input.ActionConfirm) {
		t.Fatal("did not expect just pressed from a stray button-up event")
	}
}
