package devpanel

import (
	"github.com/dhannyell/arasy/input"
	"github.com/dhannyell/arasy/math2d"
)

// eventRouter coalesces raw host events into the per-frame shapes the ui
// package reads. It mirrors what the driver does for the game, and has to be
// its own copy for a reason worth stating: the panel sees events *before* the
// driver does, and only what it declines is forwarded. Reusing the driver's
// accumulator would mean the game's input state advanced from clicks the game
// never received.
//
// The pointer and keyboard state persist across frames because a gesture
// does: a slider drag is a button that went down one frame and comes up
// several frames later.
type eventRouter struct {
	pointer  input.PointerState
	keyboard input.KeyboardState
	mapper   *input.Mapper
}

func newEventRouter() eventRouter {
	return eventRouter{
		// The panel's own widgets navigate with Tab, confirm with Enter and
		// dismiss with Escape, whatever bindings the game happens to declare.
		mapper: input.NewMapper([]input.Binding{
			{Source: input.Keyboard(input.KeyTab), Action: input.ActionFocusNext, Scale: 1},
			{Source: input.Keyboard(input.KeyEnter), Action: input.ActionConfirm, Scale: 1},
			{Source: input.Keyboard(input.KeyEscape), Action: input.ActionCancel, Scale: 1},
		}),
	}
}

// frame folds one batch of host events into this frame's pointer, keyboard
// and action state. Call endFrame once the widgets have read them.
func (r *eventRouter) frame(events []input.Event) (input.PointerFrame, input.KeyFrame, input.State) {
	frame := input.PointerFrame{
		Pos:     r.pointer.Pos(),
		Inside:  r.pointer.Inside(),
		Buttons: r.pointer.Buttons(),
	}
	keys := r.keyboard.BeginFrame()

	for _, event := range events {
		r.mapper.Apply(event)
		switch event.Source.Device {
		case input.DeviceMouse:
			frame = r.accumulatePointer(frame, event)
		case input.DeviceKeyboard:
			keys = r.accumulateKey(keys, event)
		}
	}

	return frame, keys, r.mapper.State()
}

// endFrame closes the action edges, so a key pressed this frame does not read
// as just-pressed again on the next one.
func (r *eventRouter) endFrame() { r.mapper.EndFrame() }

func (r *eventRouter) accumulateKey(keys input.KeyFrame, event input.Event) input.KeyFrame {
	keys = keys.AddEvent(event)
	switch event.Kind {
	case input.EventButtonDown:
		r.keyboard.SetDown(event.Source.Code, true)
	case input.EventButtonUp:
		r.keyboard.SetDown(event.Source.Code, false)
	}
	return keys
}

func (r *eventRouter) accumulatePointer(frame input.PointerFrame, event input.Event) input.PointerFrame {
	pos := math2d.Vec2{X: event.X, Y: event.Y}

	switch event.Kind {
	case input.EventPointerMove:
		frame.Pos = pos
		frame.Inside = true
		r.pointer.SetPos(pos)
		r.pointer.SetInside(true)
	case input.EventButtonDown, input.EventButtonUp:
		button, ok := pointerButton(event.Source.Code)
		if !ok {
			return frame
		}
		down := event.Kind == input.EventButtonDown
		frame.Pos = pos
		frame.Inside = true
		frame.Buttons[button] = down
		frame.Events = append(frame.Events, input.PointerEvent{Kind: event.Kind, Button: button, Pos: pos})
		r.pointer.SetPos(pos)
		r.pointer.SetButton(button, down)
		r.pointer.SetInside(true)
	case input.EventPointerLeave:
		frame.Inside = false
		for button, down := range r.pointer.Buttons() {
			if !down {
				continue
			}
			frame.Buttons[button] = false
			frame.Events = append(frame.Events, input.PointerEvent{
				Kind:   input.EventButtonUp,
				Button: input.PointerButton(button),
				Pos:    r.pointer.Pos(),
			})
			r.pointer.SetButton(input.PointerButton(button), false)
		}
		r.pointer.SetInside(false)
	case input.EventAxisChanged:
		switch event.Source.Code {
		case input.MouseWheelX:
			frame.Wheel.X += event.Value
		case input.MouseWheelY:
			frame.Wheel.Y += event.Value
		}
		r.pointer.SetWheel(frame.Wheel)
	}

	return frame
}

func pointerButton(code input.SourceCode) (input.PointerButton, bool) {
	switch code {
	case input.MouseLeft:
		return input.PointerPrimary, true
	case input.MouseRight:
		return input.PointerSecondary, true
	case input.MouseMiddle:
		return input.PointerMiddle, true
	default:
		return input.PointerUnknown, false
	}
}
