package input_test

import (
	"testing"

	"github.com/dhannyell/frameforge/input"
)

func TestSourceValidAcceptsMatchingDeviceAndCode(t *testing.T) {
	source := input.Source{
		Device: input.DeviceKeyboard,
		Code:   input.KeyW,
	}

	if !source.Valid() {
		t.Fatal("expected keyboard source to be valid")
	}
}

func TestSourceValidRejectsMismatchedDeviceAndCode(t *testing.T) {
	source := input.Source{
		Device: input.DeviceKeyboard,
		Code:   input.MouseLeft,
	}

	if source.Valid() {
		t.Fatal("expected keyboard source with mouse code to be invalid")
	}
}

func TestActionValidRejectsUnknownAction(t *testing.T) {
	if input.ActionUnknown.Valid() {
		t.Fatal("expected unknown action to be invalid")
	}
}

func TestBindingValidRequiresValidSourceAndAction(t *testing.T) {
	valid := input.Binding{
		Source: input.Source{
			Device: input.DeviceKeyboard,
			Code:   input.KeyD,
		},
		Action: input.ActionMoveX,
		Scale:  1,
	}

	if !valid.Valid() {
		t.Fatal("expected binding to be valid")
	}

	invalid := input.Binding{
		Source: input.Source{
			Device: input.DeviceKeyboard,
			Code:   input.MouseLeft,
		},
		Action: input.ActionMoveX,
		Scale:  1,
	}

	if invalid.Valid() {
		t.Fatal("expected binding with invalid source to be invalid")
	}
}
