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

func TestInputEventFromClientMessageMapsMouseMove(t *testing.T) {
	event, ok := inputEventFromClientMessage(clientMessage{
		Device: "mouse",
		Code:   "Position",
		Kind:   "move",
		X:      12,
		Y:      34,
	})
	if !ok {
		t.Fatal("inputEventFromClientMessage ok = false, want true")
	}

	if event.Source.Device != input.DeviceMouse || event.Source.Code != input.MouseX {
		t.Fatalf("source = %#v, want mouse MouseX", event.Source)
	}
	if event.Kind != input.EventPointerMove {
		t.Fatalf("kind = %v, want EventPointerMove", event.Kind)
	}
	if event.X != 12 || event.Y != 34 {
		t.Fatalf("position = (%v, %v), want (12, 34)", event.X, event.Y)
	}
	if !event.Valid() {
		t.Fatal("event.Valid() = false, want true")
	}
}

func TestInputEventFromClientMessageMapsMouseButton(t *testing.T) {
	event, ok := inputEventFromClientMessage(clientMessage{
		Device: "mouse",
		Code:   "Left",
		Kind:   "down",
		X:      56,
		Y:      78,
	})
	if !ok {
		t.Fatal("inputEventFromClientMessage ok = false, want true")
	}

	if event.Source.Device != input.DeviceMouse || event.Source.Code != input.MouseLeft {
		t.Fatalf("source = %#v, want mouse MouseLeft", event.Source)
	}
	if event.Kind != input.EventButtonDown {
		t.Fatalf("kind = %v, want EventButtonDown", event.Kind)
	}
	if event.X != 56 || event.Y != 78 {
		t.Fatalf("position = (%v, %v), want (56, 78)", event.X, event.Y)
	}
	if !event.Valid() {
		t.Fatal("event.Valid() = false, want true")
	}
}

func TestInputEventFromClientMessageMapsMouseLeave(t *testing.T) {
	event, ok := inputEventFromClientMessage(clientMessage{
		Device: "mouse",
		Code:   "Leave",
		Kind:   "leave",
	})
	if !ok {
		t.Fatal("inputEventFromClientMessage ok = false, want true")
	}

	if event.Source.Device != input.DeviceMouse || event.Source.Code != input.MouseUnknown {
		t.Fatalf("source = %#v, want mouse MouseUnknown", event.Source)
	}
	if event.Kind != input.EventPointerLeave {
		t.Fatalf("kind = %v, want EventPointerLeave", event.Kind)
	}
	if !event.Valid() {
		t.Fatal("event.Valid() = false, want true")
	}
}

func TestInputEventFromClientMessageMapsMouseWheel(t *testing.T) {
	event, ok := inputEventFromClientMessage(clientMessage{
		Device: "mouse",
		Code:   "WheelY",
		Kind:   "axis",
		Value:  -1.25,
		X:      12,
		Y:      34,
	})
	if !ok {
		t.Fatal("inputEventFromClientMessage ok = false, want true")
	}

	if event.Source.Device != input.DeviceMouse || event.Source.Code != input.MouseWheelY {
		t.Fatalf("source = %#v, want mouse MouseWheelY", event.Source)
	}
	if event.Kind != input.EventAxisChanged {
		t.Fatalf("kind = %v, want EventAxisChanged", event.Kind)
	}
	if event.Value != -1.25 {
		t.Fatalf("value = %v, want -1.25", event.Value)
	}
	if event.X != 12 || event.Y != 34 {
		t.Fatalf("position = (%v, %v), want (12, 34)", event.X, event.Y)
	}
	if !event.Valid() {
		t.Fatal("event.Valid() = false, want true")
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

func TestControlCommandFromMessageMapsScrubToTick(t *testing.T) {
	cmd, ok := controlCommandFromMessage(clientMessage{
		Action: "scrubToTick",
		Tick:   42,
	})
	if !ok {
		t.Fatal("controlCommandFromMessage ok = false, want true")
	}

	if cmd.Kind != runtime.ControlScrubToTick {
		t.Fatalf("kind = %v, want ControlScrubToTick", cmd.Kind)
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

func TestHubDrainInboxConsumesViewportOnce(t *testing.T) {
	h := &hub{}
	h.enqueueResize(clientMessage{Width: 640, Height: 480})

	_, _, viewport, haveViewport := h.drainInbox()
	if !haveViewport {
		t.Fatal("first drain haveViewport = false, want true")
	}
	if viewport.Width() != 640 || viewport.Height() != 480 {
		t.Fatalf("viewport = %+v, want 640x480", viewport)
	}

	_, _, _, haveViewport = h.drainInbox()
	if haveViewport {
		t.Fatal("second drain haveViewport = true, want false")
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
