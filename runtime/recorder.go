package runtime

import (
	"sort"

	"github.com/dhannyell/arasy/audio"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/scene"
	"github.com/dhannyell/arasy/ui"
)

type recorder struct {
	interval      int
	limit         int
	archiveBudget int
	keyframes     []driverSnapshot
	archive       []driverSnapshot
}

// WantsCapture avoids cloning the world on ticks that do not become keyframes —
// the Driver only assembles the driverSnapshot when the recorder is about to save it.
func (r *recorder) WantsCapture(tick uint64) bool {
	if r == nil || r.limit <= 0 || r.interval <= 0 {
		return false
	}
	return tick%uint64(r.interval) == 0
}

// driverSnapshot is the complete mutable runtime state needed for rewind.
// Every mutable collaborator must contribute a Snapshot/Restore payload here.
type driverSnapshot struct {
	world      *scene.World
	input      driverInputSnapshot
	uiState    ui.State
	gameState  GameState
	rng        Rand
	viewport   math2d.Rect
	tick       uint64
	checksum   uint64
	audioState audio.State
}

// driverSnapshotFields counts driverSnapshot's fields. Bump it when adding
// a field — and wire the field into captureSnapshot AND restoreSnapshot;
// the structural guard test fails until you do.
const driverSnapshotFields = 9

func newRecorder(historyLen, interval int) *recorder {
	return newRecorderWithArchiveBudget(historyLen, interval, 0)
}

func newRecorderWithArchiveBudget(historyLen, interval, archiveBudget int) *recorder {
	if interval <= 0 {
		interval = defaultKeyframeInterval
	}
	limit := historyLen / interval
	if historyLen > 0 && limit < 1 {
		limit = 1
	}
	if archiveBudget == 0 {
		archiveBudget = 4 * limit
		if archiveBudget < 16 {
			archiveBudget = 16
		}
	}

	archive := make([]driverSnapshot, 0)
	if archiveBudget > 0 {
		archive = make([]driverSnapshot, 0, archiveBudget)
	}
	return &recorder{
		interval:      interval,
		limit:         limit,
		archiveBudget: archiveBudget,
		keyframes:     make([]driverSnapshot, 0, limit),
		archive:       archive,
	}
}

func (r *recorder) Capture(snap driverSnapshot) {
	if snap.world == nil || snap.input.mapper == nil {
		return
	}
	if r.WantsCapture(snap.tick) {
		r.keyframes = append(r.keyframes, snap)
		if len(r.keyframes) > r.limit {
			r.archiveKeyframe(r.keyframes[0])
			copy(r.keyframes, r.keyframes[1:])
			r.keyframes = r.keyframes[:r.limit]
		}
	}
}

func (r *recorder) archiveKeyframe(snap driverSnapshot) {
	if r.archiveBudget < 0 {
		return
	}
	r.archive = append(r.archive, snap)
	r.thinArchive()
}

func (r *recorder) thinArchive() {
	if len(r.archive) <= r.archiveBudget {
		return
	}

	half := len(r.archive) / 2
	kept := r.archive[:0]
	for i, snap := range r.archive {
		if i < half && i%2 == 1 {
			continue
		}
		kept = append(kept, snap)
	}
	r.archive = kept

	// A budget of one or two does not always have an odd index in the older
	// half. Keep the budget strict for explicitly configured small archives.
	for len(r.archive) > r.archiveBudget {
		copy(r.archive, r.archive[1:])
		r.archive = r.archive[:len(r.archive)-1]
	}
}

// KeyframeAtOrBefore returns the nearest keyframe with tick ≤ target.
// It does NOT remove anything — seek is non-destructive; truncation is performed by the fork.
func (r *recorder) KeyframeAtOrBefore(target uint64) (driverSnapshot, bool) {
	if snap, ok := keyframeAtOrBefore(r.keyframes, target); ok {
		return snap, true
	}
	return keyframeAtOrBefore(r.archive, target)
}

// KeyframeAt returns the exact keyframe for the tick, if it exists (used by
// the checksum check during re-simulation).
func (r *recorder) KeyframeAt(tick uint64) (driverSnapshot, bool) {
	if snap, ok := keyframeAt(r.keyframes, tick); ok {
		return snap, true
	}
	return keyframeAt(r.archive, tick)
}

func keyframeAtOrBefore(keyframes []driverSnapshot, target uint64) (driverSnapshot, bool) {
	i := sort.Search(len(keyframes), func(i int) bool {
		return keyframes[i].tick > target
	}) - 1
	if i < 0 {
		return driverSnapshot{}, false
	}
	return keyframes[i], true
}

func keyframeAt(keyframes []driverSnapshot, tick uint64) (driverSnapshot, bool) {
	i := sort.Search(len(keyframes), func(i int) bool {
		return keyframes[i].tick >= tick
	})
	if i >= len(keyframes) || keyframes[i].tick != tick {
		return driverSnapshot{}, false
	}
	return keyframes[i], true
}

// DropAfter discards keyframes from the abandoned future. Called only by
// the fork in stepOnce, never by seek.
func (r *recorder) DropAfter(tick uint64) {
	r.keyframes = dropKeyframesAfter(r.keyframes, tick)
	r.archive = dropKeyframesAfter(r.archive, tick)
}

func dropKeyframesAfter(keyframes []driverSnapshot, tick uint64) []driverSnapshot {
	end := sort.Search(len(keyframes), func(i int) bool {
		return keyframes[i].tick > tick
	})
	return keyframes[:end]
}

func (r *recorder) Clear() {
	r.keyframes = r.keyframes[:0]
	r.archive = r.archive[:0]
}

func (r *recorder) Len() int {
	return len(r.keyframes)
}

func (r *recorder) Range() (start uint64, end uint64, ok bool) {
	if len(r.keyframes) == 0 {
		return 0, 0, false
	}

	return r.keyframes[0].tick, r.keyframes[len(r.keyframes)-1].tick, true
}
