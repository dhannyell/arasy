package scene_test

import (
	"testing"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/scene"
)

// The hop's whole contract: the entity flies the line, peaks halfway, and the
// tween is gone the tick it lands -- leaving the entity exactly at the target
// with zero height, so game code can use the component's absence as "landed".
func TestHopFliesTheArcAndLandsExactly(t *testing.T) {
	w := scene.NewWorld()
	id := w.SpawnBundle(scene.Bundle{
		Transform: &scene.Transform{Position: math2d.Vec2{X: 10, Y: 10}},
	})

	target := math2d.Vec2{X: 40, Y: 20}
	if !w.Hop(id, target, scene.HopOptions{Duration: 1, Apex: 6}) {
		t.Fatal("Hop refused an entity that has a Transform")
	}

	dt := 1.0 / 60.0
	peak := 0.0
	for range 30 {
		w.Step(dt, scene.StepConfig{})
		if h, ok := w.Height(id); ok && h.Z > peak {
			peak = h.Z
		}
	}

	// Halfway through the flight the entity is mid-line and near the apex.
	transform, _ := w.Transform(id)
	if want := (math2d.Vec2{X: 25, Y: 15}); transform.Position.Sub(want).Len() > 0.6 {
		t.Fatalf("halfway position = %v, want near %v", transform.Position, want)
	}
	if peak < 5.5 {
		t.Fatalf("peak height = %v, want it near the apex of 6", peak)
	}

	for range 40 {
		w.Step(dt, scene.StepConfig{})
	}

	if _, hopping := w.HopTween(id); hopping {
		t.Fatal("the hop should remove itself on landing")
	}
	transform, _ = w.Transform(id)
	if transform.Position != target {
		t.Fatalf("landed at %v, want exactly %v", transform.Position, target)
	}
	if h, _ := w.Height(id); h.Z != 0 {
		t.Fatalf("landed with height %v, want 0", h.Z)
	}
}
