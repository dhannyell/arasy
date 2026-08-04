package scene_test

import (
	"testing"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/scene"
)

// A 2.5D entity is Height plus Ballistics, so a bundle that cannot carry them
// forces the one kind of entity this engine is built around to be assembled
// component by component.
func TestSpawnBundleCarriesTheHeightComponents(t *testing.T) {
	world := scene.NewWorld()

	id := world.SpawnBundle(scene.Bundle{
		Transform:  &scene.Transform{Position: math2d.Vec2{X: 3, Y: 4}},
		Height:     &scene.Height{Z: 12, VZ: -2},
		Ballistics: &scene.Ballistics{Gravity: 9.8, BounceDamping: 0.5},
		Spin:       &scene.Spin{Omega: 1.5},
	})

	height, ok := world.Height(id)
	if !ok || height.Z != 12 || height.VZ != -2 {
		t.Errorf("Height = %+v (present %v), want {12 -2}", height, ok)
	}

	ballistics, ok := world.Ballistics(id)
	if !ok || ballistics.Gravity != 9.8 || ballistics.BounceDamping != 0.5 {
		t.Errorf("Ballistics = %+v (present %v)", ballistics, ok)
	}

	spin, ok := world.Spin(id)
	if !ok || spin.Omega != 1.5 {
		t.Errorf("Spin = %+v (present %v), want {1.5}", spin, ok)
	}
}
