package runtime

type GameState interface {
	isGameState()
}

type StatefulGame interface {
	GameState() GameState
	RestoreGameState(GameState)
}

type stateSnapshot[T any] struct {
	value T
}

func (stateSnapshot[T]) isGameState() {}

type StateSlot[T any] struct {
	State T
}

func (s *StateSlot[T]) GameState() GameState {
	return stateSnapshot[T]{value: s.State}
}

func (s *StateSlot[T]) RestoreGameState(state GameState) {
	if state == nil {
		var zero T
		s.State = zero
		return
	}

	s.State = state.(stateSnapshot[T]).value
}
