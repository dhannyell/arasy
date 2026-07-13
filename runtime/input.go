package runtime

import (
	"github.com/dhannyell/frameforge/input"
	"github.com/dhannyell/frameforge/math2d"
)

type driverInput struct {
	mapper    *input.Mapper
	pointer   input.PointerState
	frame     input.PointerFrame
	lastInput input.State
}

type driverInputSnapshot struct {
	mapper    *input.Mapper
	lastInput input.State
	pointer   input.PointerState
}

func newDriverInput(bindings []input.Binding) *driverInput {
	i := &driverInput{
		mapper:    input.NewMapper(withDefaultUIBindings(bindings)),
		lastInput: input.NewState(),
	}
	i.resetFrame()
	return i
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

		if event.Kind != input.EventPointerMove && event.Kind != input.EventPointerLeave {
			i.mapper.Apply(event)
		}

		if event.Source.Device == input.DeviceMouse {
			i.accumulatePointer(event)
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
	}
}

func (i *driverInput) Restore(snapshot driverInputSnapshot) {
	i.mapper = snapshot.mapper
	i.lastInput = snapshot.lastInput
	i.pointer = snapshot.pointer
	i.resetFrame()
}

func (i *driverInput) Reset() {
	i.mapper.Reset()
	i.lastInput = input.NewState()
	i.resetFrame()
}

func (i *driverInput) resetFrame() {
	i.frame = input.PointerFrame{
		Pos:     i.pointer.Pos(),
		Inside:  i.pointer.Inside(),
		Buttons: i.pointer.Buttons(),
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
