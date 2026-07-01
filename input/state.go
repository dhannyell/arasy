package input

import "maps"

type State struct {
	values       map[Action]float64
	pressed      map[Action]bool
	justPressed  map[Action]bool
	justReleased map[Action]bool
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

func (s *State) Clone() State {
	return s.clone()
}

func (s *State) clone() State {
	clone := NewState()

	maps.Copy(clone.values, s.values)

	maps.Copy(clone.pressed, s.pressed)

	maps.Copy(clone.justPressed, s.justPressed)

	maps.Copy(clone.justReleased, s.justReleased)

	return clone
}
