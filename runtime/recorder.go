package runtime

import (
	"github.com/dhannyell/frameforge/scene"
	"github.com/dhannyell/frameforge/ui"
)

type recorder struct {
	limit     int
	snapshots []driverSnapshot
}

// driverSnapshot is the complete mutable runtime state needed for rewind.
// Every mutable collaborator must contribute a Snapshot/Restore payload here.
type driverSnapshot struct {
	world     *scene.World
	input     driverInputSnapshot
	uiState   ui.State
	gameState GameState
	tick      uint64
}

func newRecorder(limit int) *recorder {
	return &recorder{
		limit:     limit,
		snapshots: make([]driverSnapshot, 0),
	}
}

func (r *recorder) Capture(snapshot driverSnapshot) {
	if r.limit <= 0 || snapshot.world == nil || snapshot.input.mapper == nil {
		return
	}

	r.snapshots = append(r.snapshots, snapshot)

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

// BackToTick restores the snapshot at or before target. currentTick is the
// live driver tick the caller is restoring from; BackToTick is a no-op when
// target is not strictly before it. Without this guard, calling BackToTick
// with the same target twice would truncate the snapshot stack twice and
// rewind one extra tick on the second call instead of staying put.
func (r *recorder) BackToTick(currentTick, target uint64) (driverSnapshot, bool) {
	if len(r.snapshots) == 0 || target >= currentTick {
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
