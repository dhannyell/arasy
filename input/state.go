package input

import "maps"

type State struct {
	values       map[Action]float64
	pressed      map[Action]bool
	justPressed  map[Action]bool
	justReleased map[Action]bool
	lastDevice   DeviceKind
}

func NewState() State {
	return State{
		values:       make(map[Action]float64),
		pressed:      make(map[Action]bool),
		justPressed:  make(map[Action]bool),
		justReleased: make(map[Action]bool),
	}
}

func (s *State) Value(action Action) float64 {
	return s.values[action]
}

func (s *State) Pressed(action Action) bool {
	return s.pressed[action]
}

func (s *State) JustPressed(action Action) bool {
	return s.justPressed[action]
}

func (s *State) JustReleased(action Action) bool {
	return s.justReleased[action]
}

// LastInputDevice reports the last device that produced meaningful
// deterministic input. Button presses, stick movement outside the configured
// deadzone, non-zero trigger/wheel axes, and pointer movement are meaningful;
// releases, neutral axes, and pointer leave are not. The value is snapshotted
// and replayed with the rest of the input state.
func (s *State) LastInputDevice() DeviceKind {
	return s.lastDevice
}

func (s *State) Clone() State {
	return s.clone()
}

func (s *State) clone() State {
	clone := NewState()
	clone.lastDevice = s.lastDevice

	maps.Copy(clone.values, s.values)

	maps.Copy(clone.pressed, s.pressed)

	maps.Copy(clone.justPressed, s.justPressed)

	maps.Copy(clone.justReleased, s.justReleased)

	return clone
}
