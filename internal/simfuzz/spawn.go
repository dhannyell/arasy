package simfuzz

import (
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/scene"
)

// spreadPos returns a deterministic, in-bounds position for the index-th entity
// that lacks an explicit Transform. It spreads entities across the bounds with
// prime-stride lattice sampling so colliders overlap and the broad-phase is
// exercised, instead of stacking every entity on one point.
func spreadPos(bounds math2d.Rect, index int) math2d.Vec2 {
	w := bounds.Width()
	h := bounds.Height()
	if w <= 0 || h <= 0 {
		return bounds.Min
	}
	iw := int(w)
	ih := int(h)
	if iw <= 0 {
		iw = 1
	}
	if ih <= 0 {
		ih = 1
	}
	nx := float64((index*37 + 11) % iw)
	ny := float64((index*53 + 23) % ih)
	return math2d.Vec2{X: bounds.Min.X + nx, Y: bounds.Min.Y + ny}
}

// SpawnFromScript creates entities in the world according to the script, in
// mask-bit order, and appends every spawned EntityID to *ids so callers (the
// fuzz game, the NaN/Inf scanner) can iterate the known live entities without
// needing public iteration on scene.World. Every component bit declared in
// script.go is handled here; TestSpawnCoversAllDecodedComponents guards that.
func SpawnFromScript(w *scene.World, s Script, bounds math2d.Rect, ids *[]scene.EntityID) {
	if bounds == (math2d.Rect{}) {
		bounds = fuzzBounds
	}
	for idx, ed := range s.Entities {
		id := w.Spawn()
		if ids != nil {
			*ids = append(*ids, id)
		}
		spawnEntity(w, ed, idx, bounds, id)
	}
}
