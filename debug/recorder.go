package debug

import "github.com/dhannyell/frameforge/scene"

type Recorder struct {
	limit     int
	snapshots []*scene.World
}

func NewRecorder(limit int) *Recorder {
	return &Recorder{
		limit:     limit,
		snapshots: make([]*scene.World, 0),
	}
}

func (r *Recorder) Capture(world *scene.World) {
	if r.limit <= 0 || world == nil {
		return
	}

	r.snapshots = append(r.snapshots, world.Clone())

	if len(r.snapshots) > r.limit {
		copy(r.snapshots, r.snapshots[1:])
		r.snapshots = r.snapshots[:r.limit]
	}
}

func (r *Recorder) Back() (*scene.World, bool) {
	if len(r.snapshots) == 0 {
		return nil, false
	}

	world := r.snapshots[len(r.snapshots)-1]
	r.snapshots = r.snapshots[:len(r.snapshots)-1]
	return world, true
}

func (r *Recorder) Clear() {
	r.snapshots = r.snapshots[:0]
}

func (r *Recorder) Len() int {
	return len(r.snapshots)
}
