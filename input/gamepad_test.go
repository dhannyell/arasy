package input_test

import (
	"math"
	"testing"

	"github.com/dhannyell/arasy/input"
)

func TestGamepadSlotCodeReservesIndependentRanges(t *testing.T) {
	code, ok := input.GamepadSlotCode(0, input.GamepadA)
	if !ok || code != input.GamepadA {
		t.Fatalf("slot zero code = %d, %v; want %d, true", code, ok, input.GamepadA)
	}

	code, ok = input.GamepadSlotCode(1, input.GamepadA)
	if !ok || code == input.GamepadA {
		t.Fatalf("slot one code = %d, %v; want a distinct valid code", code, ok)
	}
	if (input.Source{Device: input.DeviceGamepad, Code: code}).Valid() == false {
		t.Fatal("slot one code should be valid")
	}

	if _, ok := input.GamepadSlotCode(0, input.GamepadUnknown); ok {
		t.Fatal("GamepadUnknown should not be mapped")
	}
	if _, ok := input.GamepadSlotCode(-1, input.GamepadA); ok {
		t.Fatal("negative slot should not be mapped")
	}
}

func TestNormalizeGamepadAxis(t *testing.T) {
	for _, test := range []struct {
		name string
		in   float64
		want float64
	}{
		{name: "low", in: -2, want: -1},
		{name: "high", in: 2, want: 1},
		{name: "nan", in: math.NaN(), want: 0},
		{name: "valid", in: -.25, want: -.25},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := input.NormalizeGamepadAxis(test.in); got != test.want {
				t.Fatalf("NormalizeGamepadAxis(%v) = %v, want %v", test.in, got, test.want)
			}
		})
	}
}

func TestApplyGamepadDeadzoneUsesStrictlyAboveBoundary(t *testing.T) {
	if got := input.ApplyGamepadDeadzone(.2, .2); got != 0 {
		t.Fatalf("axis at deadzone = %v, want 0", got)
	}
	if got := input.ApplyGamepadDeadzone(.6, .2); math.Abs(got-.5) > 1e-12 {
		t.Fatalf("axis above deadzone = %v, want .5", got)
	}
	if got := input.ApplyGamepadDeadzone(-.6, .2); math.Abs(got+.5) > 1e-12 {
		t.Fatalf("negative axis above deadzone = %v, want -.5", got)
	}
}

func TestEffectiveGamepadDeadzoneUsesExplicitDefault(t *testing.T) {
	for _, value := range []float64{0, -1, 1, math.NaN()} {
		if got := input.EffectiveGamepadDeadzone(value); got != input.DefaultGamepadDeadzone {
			t.Fatalf("EffectiveGamepadDeadzone(%v) = %v, want %v", value, got, input.DefaultGamepadDeadzone)
		}
	}
	if got := input.EffectiveGamepadDeadzone(.25); got != .25 {
		t.Fatalf("EffectiveGamepadDeadzone(.25) = %v, want .25", got)
	}
}
