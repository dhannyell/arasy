package canvas

import (
	"testing"

	"github.com/dhannyell/frameforge/input"
	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
	"github.com/dhannyell/frameforge/runtime"
	"github.com/dhannyell/frameforge/scene"
)

func TestInputEventFromClientMessageMapsKeyboardEvents(t *testing.T) {
	event, ok := inputEventFromClientMessage(clientMessage{
		Device: "keyboard",
		Code:   "KeyW",
		Kind:   "down",
	})
	if !ok {
		t.Fatal("inputEventFromClientMessage ok = false, want true")
	}

	if event.Source.Device != input.DeviceKeyboard || event.Source.Code != input.KeyW {
		t.Fatalf("source = %#v, want keyboard KeyW", event.Source)
	}
	if event.Kind != input.EventButtonDown {
		t.Fatalf("kind = %v, want EventButtonDown", event.Kind)
	}
}

func TestInputEventFromClientMessageRejectsUnknownInput(t *testing.T) {
	if _, ok := inputEventFromClientMessage(clientMessage{
		Device: "mouse",
		Code:   "ButtonLeft",
		Kind:   "down",
	}); ok {
		t.Fatal("mouse input ok = true, want false")
	}

	if _, ok := inputEventFromClientMessage(clientMessage{
		Device: "keyboard",
		Code:   "Nope",
		Kind:   "down",
	}); ok {
		t.Fatal("unknown keyboard code ok = true, want false")
	}
}

func TestControlCommandFromMessageMapsDebugLayers(t *testing.T) {
	mask := uint32(scene.DebugColliders | scene.DebugContacts)
	cmd, ok := controlCommandFromMessage(clientMessage{
		Action: "setDebug",
		Debug:  mask,
	})
	if !ok {
		t.Fatal("controlCommandFromMessage ok = false, want true")
	}

	if cmd.Kind != runtime.ControlSetDebugLayers {
		t.Fatalf("kind = %v, want ControlSetDebugLayers", cmd.Kind)
	}
	if cmd.DebugLayers != scene.DebugLayers(mask) {
		t.Fatalf("debug layers = %v, want %v", cmd.DebugLayers, mask)
	}
}

func TestControlCommandFromMessageMapsSpeed(t *testing.T) {
	cmd, ok := controlCommandFromMessage(clientMessage{
		Action: "setSpeed",
		Speed:  0.5,
	})
	if !ok {
		t.Fatal("controlCommandFromMessage ok = false, want true")
	}

	if cmd.Kind != runtime.ControlSetSpeed {
		t.Fatalf("kind = %v, want ControlSetSpeed", cmd.Kind)
	}
	if cmd.Speed != 0.5 {
		t.Fatalf("speed = %v, want 0.5", cmd.Speed)
	}
}

func TestControlCommandFromMessageMapsBackToTick(t *testing.T) {
	cmd, ok := controlCommandFromMessage(clientMessage{
		Action: "backToTick",
		Tick:   42,
	})
	if !ok {
		t.Fatal("controlCommandFromMessage ok = false, want true")
	}

	if cmd.Kind != runtime.ControlBackToTick {
		t.Fatalf("kind = %v, want ControlBackToTick", cmd.Kind)
	}
	if cmd.Tick != 42 {
		t.Fatalf("tick = %v, want 42", cmd.Tick)
	}
}

func TestControlCommandFromMessageMapsBackSeconds(t *testing.T) {
	cmd, ok := controlCommandFromMessage(clientMessage{
		Action:  "backSeconds",
		Seconds: 1.5,
	})
	if !ok {
		t.Fatal("controlCommandFromMessage ok = false, want true")
	}

	if cmd.Kind != runtime.ControlBackSeconds {
		t.Fatalf("kind = %v, want ControlBackSeconds", cmd.Kind)
	}
	if cmd.Seconds != 1.5 {
		t.Fatalf("seconds = %v, want 1.5", cmd.Seconds)
	}
}

func TestClientCameraPreservesFrameCameraAndUsesClientViewport(t *testing.T) {
	h := &hub{}
	c := &client{
		camera: render.Camera{
			Viewport: math2d.NewRect(
				math2d.Vec2{},
				math2d.Vec2{X: 1280, Y: 720},
			),
		},
	}
	base := render.Camera{
		World: math2d.NewRect(
			math2d.Vec2{X: 5, Y: 7},
			math2d.Vec2{X: 100, Y: 80},
		),
		Viewport: math2d.NewRect(
			math2d.Vec2{},
			math2d.Vec2{X: 1, Y: 1},
		),
		Aspect: render.AspectExpand,
	}

	got := h.clientCamera(base, c)

	if got.World != base.World {
		t.Fatalf("world = %+v, want %+v", got.World, base.World)
	}
	if got.Aspect != base.Aspect {
		t.Fatalf("aspect = %v, want %v", got.Aspect, base.Aspect)
	}
	if got.Viewport.Width() != 1280 || got.Viewport.Height() != 720 {
		t.Fatalf("viewport = %+v, want 1280x720", got.Viewport)
	}
}
