package input_test

import (
	"testing"

	"github.com/dhannyell/arasy/input"
)

func TestMapperMapsKeyboardButtonToActionValue(t *testing.T) {
	mapper := input.NewMapper([]input.Binding{
		{
			Source: keyboard(input.KeyD),
			Action: input.ActionMoveX,
			Scale:  1,
		},
	})

	mapper.Apply(input.Event{
		Source: keyboard(input.KeyD),
		Kind:   input.EventButtonDown,
	})

	state := mapper.State()
	if got := state.Value(input.ActionMoveX); got != 1 {
		t.Fatalf("move x = %v, want 1", got)
	}

	if !state.Pressed(input.ActionMoveX) {
		t.Fatal("expected move x to be pressed")
	}

	if !state.JustPressed(input.ActionMoveX) {
		t.Fatal("expected move x to be just pressed")
	}
}

func TestMapperAppliesBindingScale(t *testing.T) {
	mapper := input.NewMapper([]input.Binding{
		{
			Source: keyboard(input.KeyA),
			Action: input.ActionMoveX,
			Scale:  -1,
		},
	})

	mapper.Apply(input.Event{
		Source: keyboard(input.KeyA),
		Kind:   input.EventButtonDown,
	})

	state := mapper.State()
	if got := state.Value(input.ActionMoveX); got != -1 {
		t.Fatalf("move x = %v, want -1", got)
	}
}

func TestMapperSumsAndClampsActionValues(t *testing.T) {
	mapper := input.NewMapper([]input.Binding{
		{
			Source: keyboard(input.KeyD),
			Action: input.ActionMoveX,
			Scale:  1,
		},
		{
			Source: keyboard(input.KeyArrowRight),
			Action: input.ActionMoveX,
			Scale:  1,
		},
	})

	mapper.Apply(input.Event{
		Source: keyboard(input.KeyD),
		Kind:   input.EventButtonDown,
	})
	mapper.Apply(input.Event{
		Source: keyboard(input.KeyArrowRight),
		Kind:   input.EventButtonDown,
	})

	state := mapper.State()
	if got := state.Value(input.ActionMoveX); got != 1 {
		t.Fatalf("move x = %v, want clamped value 1", got)
	}
}

func TestMapperTracksJustReleased(t *testing.T) {
	mapper := input.NewMapper([]input.Binding{
		{
			Source: keyboard(input.KeySpace),
			Action: input.ActionConfirm,
			Scale:  1,
		},
	})

	mapper.Apply(input.Event{
		Source: keyboard(input.KeySpace),
		Kind:   input.EventButtonDown,
	})

	state := mapper.State()
	if !state.JustPressed(input.ActionConfirm) {
		t.Fatal("expected confirm to be just pressed")
	}

	mapper.EndFrame()
	state = mapper.State()
	if state.JustPressed(input.ActionConfirm) {
		t.Fatal("confirm should not stay just pressed after EndFrame")
	}

	mapper.Apply(input.Event{
		Source: keyboard(input.KeySpace),
		Kind:   input.EventButtonUp,
	})

	state = mapper.State()
	if state.Pressed(input.ActionConfirm) {
		t.Fatal("confirm should not be pressed after button up")
	}

	if !state.JustReleased(input.ActionConfirm) {
		t.Fatal("expected confirm to be just released")
	}
}

func TestMapperIgnoresInvalidEventsAndBindings(t *testing.T) {
	mapper := input.NewMapper([]input.Binding{
		{
			Source: input.Source{
				Device: input.DeviceKeyboard,
				Code:   input.MouseLeft,
			},
			Action: input.ActionMoveX,
			Scale:  1,
		},
	})

	if got := len(mapper.Bindings()); got != 0 {
		t.Fatalf("mapper bindings = %d, want 0", got)
	}

	mapper.Apply(input.Event{
		Source: input.Source{
			Device: input.DeviceKeyboard,
			Code:   input.MouseLeft,
		},
		Kind: input.EventButtonDown,
	})

	state := mapper.State()
	if got := state.Value(input.ActionMoveX); got != 0 {
		t.Fatalf("move x = %v, want 0", got)
	}
}

func TestMapperMapsAxisChangedEvent(t *testing.T) {
	mapper := input.NewMapper([]input.Binding{
		{
			Source: gamepad(input.GamepadLeftStickX),
			Action: input.ActionMoveX,
			Scale:  1,
		},
	})

	mapper.Apply(input.Event{
		Source: gamepad(input.GamepadLeftStickX),
		Kind:   input.EventAxisChanged,
		Value:  0.5,
	})

	state := mapper.State()
	if got := state.Value(input.ActionMoveX); got != 0.5 {
		t.Fatalf("move x = %v, want 0.5", got)
	}
}

func keyboard(code input.SourceCode) input.Source {
	return input.Source{
		Device: input.DeviceKeyboard,
		Code:   code,
	}
}

func gamepad(code input.SourceCode) input.Source {
	return input.Source{
		Device: input.DeviceGamepad,
		Code:   code,
	}
}
