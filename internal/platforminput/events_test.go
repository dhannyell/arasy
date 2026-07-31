package platforminput_test

import (
	"testing"

	"github.com/dhannyell/arasy/input"
	"github.com/dhannyell/arasy/internal/platforminput"
)

func TestKeyEvent(t *testing.T) {
	down, ok := platforminput.KeyEvent("Space", true)
	if !ok || down.Kind != input.EventButtonDown || down.Source.Code != input.KeySpace {
		t.Fatalf("down = %+v, ok=%v", down, ok)
	}
	up, ok := platforminput.KeyEvent("Space", false)
	if !ok || up.Kind != input.EventButtonUp {
		t.Fatalf("up = %+v, ok=%v", up, ok)
	}
	if _, ok := platforminput.KeyEvent("Unknown", true); ok {
		t.Fatal("unknown key accepted")
	}
}

func TestMouseAndWheelEvents(t *testing.T) {
	button, ok := platforminput.MouseButtonEvent(0, true, 10, 20)
	if !ok || button.Source.Code != input.MouseLeft || button.X != 10 || button.Y != 20 {
		t.Fatalf("button = %+v, ok=%v", button, ok)
	}
	wheel := platforminput.WheelEvent(2, -3)
	if len(wheel) != 2 || wheel[0].Source.Code != input.MouseWheelX || wheel[1].Source.Code != input.MouseWheelY {
		t.Fatalf("wheel = %+v", wheel)
	}
	if got := platforminput.PointerLeaveEvent(); got.Kind != input.EventPointerLeave {
		t.Fatalf("leave = %+v", got)
	}
}

func TestMouseMoveEvent(t *testing.T) {
	move := platforminput.MouseMoveEvent(15, 25)

	if move.Kind != input.EventPointerMove {
		t.Fatalf("Kind = %v, want EventPointerMove", move.Kind)
	}
	if move.Source.Device != input.DeviceMouse || move.Source.Code != input.MouseX {
		t.Fatalf("Source = %+v, want mouse/MouseX", move.Source)
	}
	if move.X != 15 || move.Y != 25 {
		t.Fatalf("X,Y = %v,%v, want 15,25", move.X, move.Y)
	}
}

func TestMouseButtonEventCode(t *testing.T) {
	cases := []struct {
		code string
		want input.SourceCode
	}{
		{"Left", input.MouseLeft},
		{"Middle", input.MouseMiddle},
		{"Right", input.MouseRight},
	}

	for _, tc := range cases {
		down, ok := platforminput.MouseButtonEventCode(tc.code, true, 5, 6)
		if !ok || down.Kind != input.EventButtonDown || down.Source.Code != tc.want || down.X != 5 || down.Y != 6 {
			t.Fatalf("%s down = %+v, ok=%v, want code=%v", tc.code, down, ok, tc.want)
		}

		up, ok := platforminput.MouseButtonEventCode(tc.code, false, 5, 6)
		if !ok || up.Kind != input.EventButtonUp || up.Source.Code != tc.want {
			t.Fatalf("%s up = %+v, ok=%v, want code=%v", tc.code, up, ok, tc.want)
		}
	}

	if _, ok := platforminput.MouseButtonEventCode("Unknown", true, 0, 0); ok {
		t.Fatal("unknown mouse button code accepted")
	}
}
