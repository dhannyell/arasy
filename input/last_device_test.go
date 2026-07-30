package input_test

import (
	"math"
	"testing"

	"github.com/dhannyell/arasy/input"
)

func TestMapperTracksOnlyMeaningfulInputDevices(t *testing.T) {
	mapper := input.NewMapper(nil)

	mapper.Apply(input.Event{
		Source: keyboard(input.KeySpace),
		Kind:   input.EventButtonDown,
	})
	state := mapper.State()
	if got := state.LastInputDevice(); got != input.DeviceKeyboard {
		t.Fatalf("device after key down = %v, want keyboard", got)
	}

	for _, event := range []input.Event{
		{Source: gamepad(input.GamepadA), Kind: input.EventButtonUp},
		{Source: gamepad(input.GamepadLeftStickX), Kind: input.EventAxisChanged, Value: 0},
		{Source: gamepad(input.GamepadLeftStickX), Kind: input.EventAxisChanged, Value: .1},
		{Source: gamepad(input.GamepadLeftStickX), Kind: input.EventAxisChanged, Value: math.NaN()},
		{Source: input.Source{Device: input.DeviceMouse, Code: input.MouseUnknown}, Kind: input.EventPointerLeave},
	} {
		mapper.Apply(event)
	}
	state = mapper.State()
	if got := state.LastInputDevice(); got != input.DeviceKeyboard {
		t.Fatalf("non-meaningful events changed device to %v, want keyboard", got)
	}

	mapper.Apply(input.Event{
		Source: gamepad(input.GamepadLeftStickX),
		Kind:   input.EventAxisChanged,
		Value:  .5,
	})
	state = mapper.State()
	if got := state.LastInputDevice(); got != input.DeviceGamepad {
		t.Fatalf("device after non-zero gamepad axis = %v, want gamepad", got)
	}

	mapper.Apply(input.Event{
		Source: input.Source{Device: input.DeviceMouse, Code: input.MouseX},
		Kind:   input.EventPointerMove,
		X:      10,
		Y:      20,
	})
	state = mapper.State()
	if got := state.LastInputDevice(); got != input.DeviceMouse {
		t.Fatalf("device after pointer move = %v, want mouse", got)
	}
}

func TestMapperCloneAndResetLastInputDevice(t *testing.T) {
	mapper := input.NewMapper(nil)
	mapper.Apply(input.Event{Source: gamepad(input.GamepadA), Kind: input.EventButtonDown})

	clone := mapper.Clone()
	clonedState := clone.State()
	if got := clonedState.LastInputDevice(); got != input.DeviceGamepad {
		t.Fatalf("cloned device = %v, want gamepad", got)
	}

	mapper.Reset()
	state := mapper.State()
	if got := state.LastInputDevice(); got != input.DeviceUnknown {
		t.Fatalf("device after reset = %v, want unknown", got)
	}
	clonedState = clone.State()
	if got := clonedState.LastInputDevice(); got != input.DeviceGamepad {
		t.Fatalf("reset of original changed clone device to %v", got)
	}
}
