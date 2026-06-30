package runtime

import (
	"github.com/dhannyell/frameforge/input"
	"github.com/dhannyell/frameforge/scene"
	"github.com/dhannyell/frameforge/ui"
)

type recorder struct {
	limit     int
	snapshots []driverSnapshot
}

type driverSnapshot struct {
	world     *scene.World
	mapper    *input.Mapper
	lastInput input.State
	uiState   ui.State
	pointer   input.PointerState
	tick      uint64
}

func newRecorder(limit int) *recorder {
	return &recorder{
		limit:     limit,
		snapshots: make([]driverSnapshot, 0),
	}
}

func (r *recorder) Capture(world *scene.World, mapper *input.Mapper, lastInput input.State, uiState ui.State, pointer input.PointerState, tick uint64) {
	if r.limit <= 0 || world == nil || mapper == nil {
		return
	}

	r.snapshots = append(r.snapshots, driverSnapshot{
		world:     world.Clone(),
		mapper:    mapper.Clone(),
		lastInput: lastInput.Clone(),
		uiState:   uiState.Clone(),
		pointer:   pointer.Clone(),
		tick:      tick,
	})

	if len(r.snapshots) > r.limit {
		copy(r.snapshots, r.snapshots[1:])
		r.snapshots = r.snapshots[:r.limit]
	}
}

func (r *recorder) Back() (driverSnapshot, bool) {
	if len(r.snapshots) == 0 {
		return driverSnapshot{}, false
	}

	snapshot := r.snapshots[len(r.snapshots)-1]
	r.snapshots = r.snapshots[:len(r.snapshots)-1]
	return snapshot, true
}

func (r *recorder) BackToTick(target uint64) (driverSnapshot, bool) {
	if len(r.snapshots) == 0 {
		return driverSnapshot{}, false
	}

	index := -1
	for i := len(r.snapshots) - 1; i >= 0; i-- {
		if r.snapshots[i].tick <= target {
			index = i
			break
		}
	}
	if index == -1 {
		index = 0
	}

	snapshot := r.snapshots[index]
	r.snapshots = r.snapshots[:index]
	return snapshot, true
}

func (r *recorder) Clear() {
	r.snapshots = r.snapshots[:0]
}

func (r *recorder) Len() int {
	return len(r.snapshots)
}

func (r *recorder) Range() (start uint64, end uint64, ok bool) {
	if len(r.snapshots) == 0 {
		return 0, 0, false
	}

	return r.snapshots[0].tick, r.snapshots[len(r.snapshots)-1].tick, true
}
