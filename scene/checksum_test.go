package scene_test

import (
	"reflect"
	"testing"

	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/scene"
)

func TestChecksumDeterministic(t *testing.T) {
	w := scene.NewWorld()
	spawnAt(w, math2d.Vec2{X: 10, Y: 20})
	spawnAt(w, math2d.Vec2{X: 30, Y: 40})

	a := w.Checksum()
	b := w.Checksum()

	if a != b {
		t.Fatalf("same world produced different checksums: %d vs %d", a, b)
	}
}

func TestChecksumDiffersOnPosition(t *testing.T) {
	w1 := scene.NewWorld()
	spawnAt(w1, math2d.Vec2{X: 10, Y: 20})

	w2 := scene.NewWorld()
	spawnAt(w2, math2d.Vec2{X: 10, Y: 21})

	if w1.Checksum() == w2.Checksum() {
		t.Fatal("worlds with different positions produced the same checksum")
	}
}

func TestChecksumDiffersOnVelocity(t *testing.T) {
	w1 := scene.NewWorld()
	id1 := spawnAt(w1, math2d.Vec2{X: 5, Y: 5})
	w1.SetVelocity(id1, scene.Velocity{Linear: math2d.Vec2{X: 1, Y: 0}})

	w2 := scene.NewWorld()
	id2 := spawnAt(w2, math2d.Vec2{X: 5, Y: 5})
	w2.SetVelocity(id2, scene.Velocity{Linear: math2d.Vec2{X: 2, Y: 0}})

	if w1.Checksum() == w2.Checksum() {
		t.Fatal("worlds with different velocities produced the same checksum")
	}
}

func TestChecksumIdenticalClone(t *testing.T) {
	w := scene.NewWorld()
	id := spawnAt(w, math2d.Vec2{X: 7, Y: 3})
	w.SetVelocity(id, scene.Velocity{Linear: math2d.Vec2{X: 5, Y: -2}})

	clone := w.Clone()
	if w.Checksum() != clone.Checksum() {
		t.Fatal("cloned world produced a different checksum than the original")
	}
}

func TestChecksumCoversAllComponentMaps(t *testing.T) {
	wt := reflect.TypeFor[scene.World]()
	count := 0
	for f := range wt.Fields() {
		if f.Type.Kind() == reflect.Map &&
			f.Type.Key() == reflect.TypeFor[scene.EntityID]() {
			count++
		}
	}
	if count != scene.ChecksumComponentMaps {
		t.Fatalf("wrong number of component maps: %d != %d", count, scene.ChecksumComponentMaps)
	}
}

func spawnAt(w *scene.World, pos math2d.Vec2) scene.EntityID {
	id := w.Spawn()
	w.SetTransform(id, scene.Transform{Position: pos, Scale: math2d.Vec2{X: 1, Y: 1}})
	return id
}
