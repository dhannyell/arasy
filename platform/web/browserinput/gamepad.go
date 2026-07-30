package browserinput

// StickyGamepadSelector owns the browser index assigned to gamepad slot zero.
// A selected index remains assigned while supported; losing it produces one
// absent observation before another supported index can be selected.
type StickyGamepadSelector struct {
	selectedPlusOne int
}

// Select observes which browser indices are connected standard-mapped
// gamepads and returns the index assigned to slot zero.
func (s *StickyGamepadSelector) Select(supported []bool) (int, bool) {
	if s.selectedPlusOne != 0 {
		selected := s.selectedPlusOne - 1
		if selected < len(supported) && supported[selected] {
			return selected, true
		}
		s.selectedPlusOne = 0
		return 0, false
	}

	for index, ok := range supported {
		if ok {
			s.selectedPlusOne = index + 1
			return index, true
		}
	}
	return 0, false
}
