package browserinput

// TouchPhase identifies the lifecycle transition of one DOM touch pointer.
type TouchPhase uint8

const (
	TouchBegin TouchPhase = iota
	TouchMove
	TouchEnd
)

// SingleTouch admits exactly one touch pointer at a time. A pointer accepted
// at TouchBegin owns the sequence until its TouchEnd; every other pointer is
// ignored without changing the active sequence.
type SingleTouch struct {
	active    bool
	pointerID int
}

// Accept reports whether pointerID owns this phase and advances the ownership
// state for begin/end transitions.
func (s *SingleTouch) Accept(pointerID int, phase TouchPhase) bool {
	switch phase {
	case TouchBegin:
		if s.active {
			return false
		}
		s.active = true
		s.pointerID = pointerID
		return true
	case TouchMove:
		return s.active && s.pointerID == pointerID
	case TouchEnd:
		if !s.active || s.pointerID != pointerID {
			return false
		}
		s.active = false
		s.pointerID = 0
		return true
	default:
		return false
	}
}
