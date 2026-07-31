package runtime

import (
	"github.com/dhannyell/arasy/input"
	"github.com/dhannyell/arasy/math2d"
)

type driverInput struct {
	mapper    *input.Mapper
	pointer   input.PointerState
	keyboard  input.KeyboardState
	frame     input.PointerFrame
	keys      input.KeyFrame
	lastInput input.State
	// held is every source currently pressed, rebuilt from the same events
	// the journal replays. It rewinds with the rest of the input state, so it
	// always describes the presses of the tick the driver is actually on.
	held []heldInput
	// gamepad mirrors raw pad state for the debug overlay only. It is
	// deliberately outside Snapshot/Restore: it describes the physical
	// controller, not the simulation.
	gamepad debugGamepadState
}

type driverInputSnapshot struct {
	mapper    *input.Mapper
	lastInput input.State
	pointer   input.PointerState
	keyboard  input.KeyboardState
	held      []heldInput
}

func newDriverInput(bindings []input.Binding) *driverInput {
	return newDriverInputWithDeadzone(bindings, 0)
}

func newDriverInputWithDeadzone(bindings []input.Binding, deadzone float64) *driverInput {
	i := &driverInput{
		mapper:    input.NewMapperWithDeadzone(withDefaultUIBindings(bindings), deadzone),
		lastInput: input.NewState(),
	}
	i.resetFrame()
	return i
}

func (i *driverInput) SetGamepadDeadzone(deadzone float64) {
	i.mapper.SetGamepadDeadzone(deadzone)
}

func withDefaultUIBindings(bindings []input.Binding) []input.Binding {
	defaults := []input.Binding{
		{Source: input.Keyboard(input.KeyTab), Action: input.ActionFocusNext, Scale: 1},
		{Source: input.Keyboard(input.KeyEnter), Action: input.ActionConfirm, Scale: 1},
		{Source: input.Keyboard(input.KeyEscape), Action: input.ActionCancel, Scale: 1},
	}
	return append(append([]input.Binding(nil), bindings...), defaults...)
}

func (i *driverInput) Apply(events []input.Event, tick uint64) {
	for _, event := range events {
		event.Tick = tick

		i.mapper.Apply(event)
		i.track(event)

		switch event.Source.Device {
		case input.DeviceMouse:
			i.accumulatePointer(event)
		case input.DeviceKeyboard:
			i.accumulateKey(event)
		case input.DeviceGamepad:
			i.gamepad.apply(event)
		}
	}
}

func (i *driverInput) BeginTick() input.State {
	i.lastInput = i.mapper.State()
	return i.lastInput
}

func (i *driverInput) PointerFrame() input.PointerFrame {
	return i.frame
}

// KeyFrame returns the tick's typed characters and editing-key edges. It is
// what text widgets read: Mapper answers what the game means by a key, this
// answers which characters arrived.
func (i *driverInput) KeyFrame() input.KeyFrame {
	return i.keys
}

func (i *driverInput) EndTick(tick uint64) {
	i.resetFrame()
	i.mapper.EndFrame()
}

func (i *driverInput) Pointer() input.PointerState {
	return i.pointer
}

func (i *driverInput) LastInput() input.State {
	return i.lastInput
}

func (i *driverInput) Bindings() []input.Binding {
	return i.mapper.Bindings()
}

func (i *driverInput) Snapshot() driverInputSnapshot {
	return driverInputSnapshot{
		mapper:    i.mapper.Clone(),
		lastInput: i.lastInput.Clone(),
		pointer:   i.pointer.Clone(),
		keyboard:  i.keyboard.Clone(),
		held:      append([]heldInput(nil), i.held...),
	}
}

func (i *driverInput) Restore(snapshot driverInputSnapshot) {
	i.mapper = snapshot.mapper
	i.lastInput = snapshot.lastInput
	i.pointer = snapshot.pointer
	i.keyboard = snapshot.keyboard
	i.held = append([]heldInput(nil), snapshot.held...)
	i.resetFrame()
}

func (i *driverInput) Reset() {
	i.mapper.Reset()
	i.lastInput = input.NewState()
	i.keyboard = input.KeyboardState{}
	i.held = nil
	// The gamepad mirror is deliberately not cleared here. Reset rewinds the
	// simulation; it does not unplug the controller, and a readout that blanked
	// on every scrub would be describing the wrong thing.
	i.resetFrame()
}

func (i *driverInput) resetFrame() {
	i.frame = input.PointerFrame{
		Pos:     i.pointer.Pos(),
		Inside:  i.pointer.Inside(),
		Buttons: i.pointer.Buttons(),
	}
	i.keys = i.keyboard.BeginFrame()
}

// accumulateKey folds a keyboard event into the tick's frame and the held-key
// state that outlives it. Text arrives here too: it is keyboard input the
// Mapper has no use for.
func (i *driverInput) accumulateKey(e input.Event) {
	i.keys = i.keys.AddEvent(e)
	switch e.Kind {
	case input.EventButtonDown:
		i.keyboard.SetDown(e.Source.Code, true)
	case input.EventButtonUp:
		i.keyboard.SetDown(e.Source.Code, false)
	}
}

func (i *driverInput) accumulatePointer(e input.Event) {
	pos := math2d.Vec2{X: e.X, Y: e.Y}

	switch e.Kind {
	case input.EventPointerMove:
		i.frame.Pos = pos
		i.frame.Inside = true
		i.pointer.SetPos(pos)
		i.pointer.SetInside(true)
	case input.EventButtonDown:
		if btn, ok := mouseButton(e.Source.Code); ok {
			i.frame.Pos = pos
			i.frame.Inside = true
			i.frame.Buttons[btn] = true
			i.frame.Events = append(i.frame.Events, input.PointerEvent{
				Kind: e.Kind, Button: btn, Pos: pos,
			})
			i.pointer.SetPos(pos)
			i.pointer.SetButton(btn, true)
			i.pointer.SetInside(true)
		}
	case input.EventButtonUp:
		if btn, ok := mouseButton(e.Source.Code); ok {
			i.frame.Pos = pos
			i.frame.Inside = true
			i.frame.Buttons[btn] = false
			i.frame.Events = append(i.frame.Events, input.PointerEvent{
				Kind: e.Kind, Button: btn, Pos: pos,
			})
			i.pointer.SetPos(pos)
			i.pointer.SetButton(btn, false)
			i.pointer.SetInside(true)
		}
	case input.EventPointerLeave:
		i.frame.Inside = false
		for btn, down := range i.pointer.Buttons() {
			if down {
				i.frame.Buttons[btn] = false
				i.frame.Events = append(i.frame.Events, input.PointerEvent{
					Kind: input.EventButtonUp, Button: input.PointerButton(btn), Pos: i.pointer.Pos(),
				})
				i.pointer.SetButton(input.PointerButton(btn), false)
			}
		}
		i.pointer.SetInside(false)
	case input.EventAxisChanged:
		switch e.Source.Code {
		case input.MouseWheelX:
			i.frame.Wheel.X += e.Value
			i.pointer.SetWheel(i.frame.Wheel)
		case input.MouseWheelY:
			i.frame.Wheel.Y += e.Value
			i.pointer.SetWheel(i.frame.Wheel)
		default:
			i.frame.Inside = false
		}
	default:
		i.frame.Inside = false
	}
}

func mouseButton(code input.SourceCode) (input.PointerButton, bool) {
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
