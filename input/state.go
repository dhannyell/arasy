package input

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

func (s State) clone() State {
	clone := NewState()

	for action, value := range s.values {
		clone.values[action] = value
	}

	for action, pressed := range s.pressed {
		clone.pressed[action] = pressed
	}

	for action, justPressed := range s.justPressed {
		clone.justPressed[action] = justPressed
	}

	for action, justReleased := range s.justReleased {
		clone.justReleased[action] = justReleased
	}

	return clone
}
