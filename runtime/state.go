package runtime

type GameState interface {
	isGameState()
	// debugValue exposes the boxed value to development tooling through
	// DebugStateValue. Unexported, like isGameState, so only this package can
	// implement GameState.
	debugValue() any
}

type StatefulGame interface {
	GameState() GameState
	RestoreGameState(GameState)
}

// Cloneable is an opt-in hook for T. StateSlot[T].GameState() boxes T by
// value, which is only a real snapshot when T has no slice/map/pointer
// fields — those share their backing storage across every boxed copy, so
// mutating current state after a snapshot corrupts the "historical" one too.
// Implement Clone() on T to deep-copy such fields; StateSlot uses it when
// present — on capture and on restore — and falls back to the plain value
// copy otherwise.
type Cloneable[T any] interface {
	Clone() T
}

type stateSnapshot[T any] struct {
	value T
}

func (stateSnapshot[T]) isGameState() {}

func (s stateSnapshot[T]) debugValue() any { return s.value }

// DebugStateValue unwraps the value a GameState boxes, for development
// tooling (the dev panel's inspector). The value shares the snapshot's
// ownership: read it, never mutate it. Nil state yields nil.
func DebugStateValue(state GameState) any {
	if state == nil {
		return nil
	}
	return state.debugValue()
}

type StateSlot[T any] struct {
	State T
}

func (s *StateSlot[T]) GameState() GameState {
	value := s.State
	if cloneable, ok := any(value).(Cloneable[T]); ok {
		value = cloneable.Clone()
	}
	return stateSnapshot[T]{value: value}
}

func (s *StateSlot[T]) RestoreGameState(state GameState) {
	if state == nil {
		var zero T
		s.State = zero
		return
	}

	value := state.(stateSnapshot[T]).value
	// Clone on restore too: handing the snapshot's reference fields to the
	// live game would let post-restore mutations corrupt the keyframe the
	// recorder still holds, so a later seek to the same tick restores
	// corrupted history. Capture clones for the same reason.
	if cloneable, ok := any(value).(Cloneable[T]); ok {
		value = cloneable.Clone()
	}
	s.State = value
}
