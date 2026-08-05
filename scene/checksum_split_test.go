package scene_test

import (
	"testing"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
	"github.com/dhannyell/arasy/scene"
)

func spawnColored(w *scene.World, color render.Color) scene.EntityID {
	id := w.Spawn()
	w.SetTransform(id, scene.Transform{Position: math2d.Vec2{X: 10, Y: 20}, Scale: math2d.Vec2{X: 1, Y: 1}})
	w.SetVelocity(id, scene.Velocity{Linear: math2d.Vec2{X: 1, Y: 0}})
	w.SetCircleSprite(id, scene.CircleSprite{
		BaseSprite: scene.BaseSprite{Layer: 1, Color: color},
		Radius:     3,
	})
	return id
}

// The reason the split exists: retuning authored visuals must not invalidate
// replay anchors, while the full checksum keeps seeing everything.
func TestSimChecksumIgnoresSpriteRecolor(t *testing.T) {
	w1 := scene.NewWorld()
	spawnColored(w1, render.RGBA(1, 0, 0, 1))

	w2 := scene.NewWorld()
	spawnColored(w2, render.RGBA(0, 1, 0, 1))

	if w1.SimChecksum() != w2.SimChecksum() {
		t.Fatal("a recolor changed SimChecksum; replay anchors would break on an aesthetic edit")
	}
	if w1.ContentChecksum() == w2.ContentChecksum() {
		t.Fatal("a recolor did not change ContentChecksum; the content warning would never fire")
	}
	if w1.Checksum() == w2.Checksum() {
		t.Fatal("a recolor did not change the full Checksum; rewind parity lost coverage")
	}
}

func TestContentChecksumIgnoresSimulation(t *testing.T) {
	w1 := scene.NewWorld()
	spawnColored(w1, render.RGBA(1, 0, 0, 1))

	w2 := scene.NewWorld()
	id := spawnColored(w2, render.RGBA(1, 0, 0, 1))
	w2.SetTransform(id, scene.Transform{Position: math2d.Vec2{X: 99, Y: 20}, Scale: math2d.Vec2{X: 1, Y: 1}})

	if w1.SimChecksum() == w2.SimChecksum() {
		t.Fatal("a moved entity did not change SimChecksum")
	}
	if w1.ContentChecksum() != w2.ContentChecksum() {
		t.Fatal("a moved entity changed ContentChecksum; content is supposed to be position-free")
	}
}

// Pins the decided line for the ambiguous pair: the tween that animates a
// sprite is simulation state (Step advances it), the sprite it writes is not.
func TestSpriteTweenIsSimulationState(t *testing.T) {
	tween := func(duration float64) *scene.World {
		w := scene.NewWorld()
		id := spawnColored(w, render.RGBA(1, 0, 0, 1))
		w.SetCircleSpriteTween(id, scene.CircleSpriteTween{
			From:     scene.CircleSprite{Radius: 2},
			To:       scene.CircleSprite{Radius: 4},
			Duration: duration,
		})
		return w
	}

	w1, w2 := tween(1), tween(2)
	if w1.SimChecksum() == w2.SimChecksum() {
		t.Fatal("differing sprite tweens produced the same SimChecksum; the deterministic clock is uncovered")
	}
	if w1.ContentChecksum() != w2.ContentChecksum() {
		t.Fatal("a sprite tween leaked into ContentChecksum")
	}
}
