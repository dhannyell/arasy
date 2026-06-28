package runtime

import (
	"github.com/dhannyell/frameforge/input"
	"github.com/dhannyell/frameforge/scene"
)

type recorder struct {
	limit     int
	snapshots []driverSnapshot
}

type driverSnapshot struct {
	world     *scene.World
	mapper    *input.Mapper
	lastInput input.State
	tick      uint64
}

func newRecorder(limit int) *recorder {
	return &recorder{
		limit:     limit,
		snapshots: make([]driverSnapshot, 0),
	}
}

func (r *recorder) Capture(world *scene.World, mapper *input.Mapper, lastInput input.State, tick uint64) {
	if r.limit <= 0 || world == nil || mapper == nil {
		return
	}

	r.snapshots = append(r.snapshots, driverSnapshot{
		world:     world.Clone(),
		mapper:    mapper.Clone(),
		lastInput: lastInput.Clone(),
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

func (r *recorder) Clear() {
	r.snapshots = r.snapshots[:0]
}

func (r *recorder) Len() int {
	return len(r.snapshots)
}
