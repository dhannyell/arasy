//go:build windows && amd64

package native

import (
	"reflect"
	"testing"

	"github.com/dhannyell/arasy/input"
	"github.com/go-gl/glfw/v3.4/glfw"
)

func TestTranslateKey(t *testing.T) {
	tests := []struct {
		key  glfw.Key
		want input.SourceCode
	}{
		{glfw.KeyA, input.KeyA}, {glfw.KeyB, input.KeyB}, {glfw.KeyC, input.KeyC}, {glfw.KeyD, input.KeyD},
		{glfw.KeyE, input.KeyE}, {glfw.KeyF, input.KeyF}, {glfw.KeyG, input.KeyG}, {glfw.KeyH, input.KeyH},
		{glfw.KeyI, input.KeyI}, {glfw.KeyJ, input.KeyJ}, {glfw.KeyK, input.KeyK}, {glfw.KeyL, input.KeyL},
		{glfw.KeyM, input.KeyM}, {glfw.KeyN, input.KeyN}, {glfw.KeyO, input.KeyO}, {glfw.KeyP, input.KeyP},
		{glfw.KeyQ, input.KeyQ}, {glfw.KeyR, input.KeyR}, {glfw.KeyS, input.KeyS}, {glfw.KeyT, input.KeyT},
		{glfw.KeyU, input.KeyU}, {glfw.KeyV, input.KeyV}, {glfw.KeyW, input.KeyW}, {glfw.KeyX, input.KeyX},
		{glfw.KeyY, input.KeyY}, {glfw.KeyZ, input.KeyZ}, {glfw.KeySpace, input.KeySpace}, {glfw.KeyTab, input.KeyTab},
		{glfw.KeyEnter, input.KeyEnter}, {glfw.KeyEscape, input.KeyEscape}, {glfw.KeyUp, input.KeyArrowUp},
		{glfw.KeyDown, input.KeyArrowDown}, {glfw.KeyLeft, input.KeyArrowLeft}, {glfw.KeyRight, input.KeyArrowRight},
	}
	for _, test := range tests {
		got, ok := translateKey(test.key)
		if !ok || got != test.want {
			t.Fatalf("translateKey(%v) = (%v, %t), want (%v, true)", test.key, got, ok, test.want)
		}
	}
	if _, ok := translateKey(glfw.Key0); ok {
		t.Fatal("translateKey(Key0) mapped unsupported key")
	}
}

func TestTranslateMouseButton(t *testing.T) {
	tests := []struct {
		button glfw.MouseButton
		want   input.SourceCode
	}{
		{glfw.MouseButtonLeft, input.MouseLeft},
		{glfw.MouseButtonRight, input.MouseRight},
		{glfw.MouseButtonMiddle, input.MouseMiddle},
	}
	for _, test := range tests {
		got, ok := translateMouseButton(test.button)
		if !ok || got != test.want {
			t.Fatalf("translateMouseButton(%v) = (%v, %t), want (%v, true)", test.button, got, ok, test.want)
		}
	}
	if _, ok := translateMouseButton(glfw.MouseButton4); ok {
		t.Fatal("translateMouseButton(MouseButton4) mapped unsupported button")
	}
}

func TestInputEvents(t *testing.T) {
	for _, test := range []struct {
		name  string
		event input.Event
		want  input.EventKind
	}{
		{"key down", keyEvent(input.KeyA, true), input.EventButtonDown},
		{"key up", keyEvent(input.KeyA, false), input.EventButtonUp},
		{"mouse down", mouseButtonEvent(input.MouseLeft, true, 4, 5), input.EventButtonDown},
		{"mouse up", mouseButtonEvent(input.MouseLeft, false, 4, 5), input.EventButtonUp},
		{"pointer move", pointerMoveEvent(4, 5), input.EventPointerMove},
		{"pointer leave", pointerLeaveEvent(), input.EventPointerLeave},
	} {
		t.Run(test.name, func(t *testing.T) {
			if test.event.Kind != test.want || !test.event.Valid() {
				t.Fatalf("event = %#v, want valid %v", test.event, test.want)
			}
		})
	}
}

func TestWheelEvents(t *testing.T) {
	got := wheelEvents(1.5, -2, 4, 5)
	want := []input.Event{
		{Source: input.Source{Device: input.DeviceMouse, Code: input.MouseWheelX}, Kind: input.EventAxisChanged, Value: 1.5, X: 4, Y: 5},
		{Source: input.Source{Device: input.DeviceMouse, Code: input.MouseWheelY}, Kind: input.EventAxisChanged, Value: -2, X: 4, Y: 5},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("wheelEvents() = %#v, want %#v", got, want)
	}
	for _, event := range got {
		if !event.Valid() {
			t.Fatalf("wheel event is invalid: %#v", event)
		}
	}
	if got := wheelEvents(0, 0, 4, 5); len(got) != 0 {
		t.Fatalf("wheelEvents(0, 0) = %#v, want no events", got)
	}
}

func TestScaleCursor(t *testing.T) {
	tests := []struct {
		name                 string
		x, y                 float64
		winW, winH, fbW, fbH int
		wantX, wantY         float64
	}{
		{name: "one to one", x: 24, y: 12, winW: 100, winH: 50, fbW: 100, fbH: 50, wantX: 24, wantY: 12},
		{name: "hidpi", x: 24, y: 12, winW: 100, winH: 50, fbW: 200, fbH: 100, wantX: 48, wantY: 24},
		{name: "zero width", x: 24, y: 12, winH: 50, fbW: 200, fbH: 100},
		{name: "zero height", x: 24, y: 12, winW: 100, fbW: 200, fbH: 100},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gotX, gotY := scaleCursor(test.x, test.y, test.winW, test.winH, test.fbW, test.fbH)
			if gotX != test.wantX || gotY != test.wantY {
				t.Fatalf("scaleCursor() = (%v, %v), want (%v, %v)", gotX, gotY, test.wantX, test.wantY)
			}
		})
	}
}

func TestGamepadEventsTranslateButtonAndAxisChanges(t *testing.T) {
	previous := gamepadSnapshot{}
	current := gamepadSnapshot{}
	current.Buttons[0] = true
	current.Axes[0] = .75

	events := gamepadEvents(previous, current)
	if len(events) != 2 {
		t.Fatalf("gamepad events = %d, want 2", len(events))
	}
	if events[0].Source.Code != input.GamepadA || events[0].Kind != input.EventButtonDown {
		t.Fatalf("button event = %+v, want A down", events[0])
	}
	if events[1].Source.Code != input.GamepadLeftStickX || events[1].Kind != input.EventAxisChanged || events[1].Value != .75 {
		t.Fatalf("axis event = %+v, want left X .75", events[1])
	}
}

func TestGamepadEventsSuppressJitterAndReleaseDisconnect(t *testing.T) {
	previous := gamepadSnapshot{Buttons: [8]bool{true}, Axes: [6]float64{.5}}
	current := gamepadSnapshot{Axes: [6]float64{.5005}}
	if events := gamepadEvents(previous, current); len(events) != 1 || events[0].Kind != input.EventButtonUp {
		t.Fatalf("jitter/disconnect events = %+v, want only button release", events)
	}
}
