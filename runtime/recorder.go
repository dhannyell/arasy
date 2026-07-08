package runtime

import (
	"sort"

	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/scene"
	"github.com/dhannyell/frameforge/ui"
)

type recorder struct {
	interval  int
	limit     int
	keyframes []driverSnapshot
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
	world     *scene.World
	input     driverInputSnapshot
	uiState   ui.State
	gameState GameState
	rng       Rand
	viewport  math2d.Rect
	tick      uint64
	checksum  uint64
}

func newRecorder(historyLen, interval int) *recorder {
	if interval <= 0 {
		interval = defaultKeyframeInterval
	}
	limit := historyLen / interval
	if historyLen > 0 && limit < 1 {
		limit = 1
	}
	return &recorder{
		interval:  interval,
		limit:     limit,
		keyframes: make([]driverSnapshot, 0),
	}
}

func (r *recorder) Capture(snap driverSnapshot) {
	if snap.world == nil || snap.input.mapper == nil {
		return
	}
	if r.WantsCapture(snap.tick) {
		r.keyframes = append(r.keyframes, snap)
		if len(r.keyframes) > r.limit {
			copy(r.keyframes, r.keyframes[1:])
			r.keyframes = r.keyframes[:r.limit]
		}
	}
}

// KeyframeAtOrBefore returns the nearest keyframe with tick ≤ target.
// It does NOT remove anything — seek is non-destructive; truncation is performed by the fork.
func (r *recorder) KeyframeAtOrBefore(target uint64) (driverSnapshot, bool) {
	i := sort.Search(len(r.keyframes), func(i int) bool {
		return r.keyframes[i].tick > target
	}) - 1
	if i < 0 {
		return driverSnapshot{}, false
	}
	return r.keyframes[i], true
}

// KeyframeAt returns the exact keyframe for the tick, if it exists (used by
// the checksum check during re-simulation).
func (r *recorder) KeyframeAt(tick uint64) (driverSnapshot, bool) {
	i := sort.Search(len(r.keyframes), func(i int) bool {
		return r.keyframes[i].tick >= tick
	})
	if i >= len(r.keyframes) || r.keyframes[i].tick != tick {
		return driverSnapshot{}, false
	}
	return r.keyframes[i], true
}

// DropAfter discards keyframes from the abandoned future. Called only by
// the fork in stepOnce, never by seek.
func (r *recorder) DropAfter(tick uint64) {
	for len(r.keyframes) > 0 && r.keyframes[len(r.keyframes)-1].tick > tick {
		r.keyframes = r.keyframes[:len(r.keyframes)-1]
	}
}

func (r *recorder) Clear() {
	r.keyframes = r.keyframes[:0]
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
