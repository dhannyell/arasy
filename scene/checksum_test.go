package scene_test

import (
	"reflect"
	"testing"

	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
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

// buildChecksumFixture spawns two entities carrying every component the
// checksum covers: circle spans the circle-shaped components (sprite,
// collider) plus the physics components, rect spans the rect-shaped ones.
// Splitting them keeps each mutation in TestChecksumChangesPerComponent
// targeted at a single field without a circle/rect collider ambiguity on one
// entity (D1 precedence is a collision-resolution rule, not a checksum one,
// but keeping them separate mirrors how the components are actually used).
func buildChecksumFixture(w *scene.World) (circle, rect scene.EntityID) {
	circle = w.Spawn()
	w.SetTransform(circle, scene.Transform{
		Position: math2d.Vec2{X: 1, Y: 2},
		Rotation: 0.5,
		Scale:    math2d.Vec2{X: 1, Y: 1},
	})
	w.SetCircleSprite(circle, scene.CircleSprite{
		BaseSprite: scene.BaseSprite{Layer: 1, Color: render.RGBA(0.1, 0.2, 0.3, 1)},
		Radius:     2,
	})
	w.SetVelocity(circle, scene.Velocity{Linear: math2d.Vec2{X: 1, Y: 0}})
	w.SetAcceleration(circle, scene.Acceleration{Linear: math2d.Vec2{X: 0, Y: 1}})
	w.SetDamping(circle, scene.Damping{Linear: 0.1})
	w.SetBody(circle, scene.Body{InvMass: 1, Restitution: 0.5})
	w.SetCircleCollider(circle, scene.CircleCollider{Radius: 2})

	rect = w.Spawn()
	w.SetTransform(rect, scene.Transform{
		Position: math2d.Vec2{X: 10, Y: 20},
		Scale:    math2d.Vec2{X: 1, Y: 1},
	})
	w.SetRectSprite(rect, scene.RectSprite{
		BaseSprite:  scene.BaseSprite{Layer: 2, Color: render.RGBA(0.4, 0.5, 0.6, 1)},
		Rect:        math2d.NewRect(math2d.Vec2{X: -2, Y: -2}, math2d.Vec2{X: 2, Y: 2}),
		Stroke:      render.RGBA(0, 0, 0, 1),
		StrokeWidth: 1,
	})
	w.SetRectCollider(rect, scene.RectCollider{HalfExtends: math2d.Vec2{X: 2, Y: 2}})

	return circle, rect
}

// TestChecksumChangesPerComponent proves every component the checksum covers
// is actually sensitive to mutation: a bug that forgot to hash a field (or
// hashed the wrong one) would leave the checksum unchanged here even though
// TestChecksumCoversAllComponentMaps stays green, since that guard only
// counts maps, not fields.
func TestChecksumChangesPerComponent(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(w *scene.World, circle, rect scene.EntityID)
	}{
		{
			name: "transform.Rotation",
			mutate: func(w *scene.World, circle, _ scene.EntityID) {
				transform, _ := w.Transform(circle)
				transform.Rotation += 1
				w.SetTransform(circle, transform)
			},
		},
		{
			name: "circleSprite.Color",
			mutate: func(w *scene.World, circle, _ scene.EntityID) {
				sprite, _ := w.CircleSprite(circle)
				sprite.Color = render.RGBA(0.9, 0.9, 0.9, 1)
				w.SetCircleSprite(circle, sprite)
			},
		},
		{
			name: "rectSprite.StrokeWidth",
			mutate: func(w *scene.World, _, rect scene.EntityID) {
				sprite, _ := w.RectSprite(rect)
				sprite.StrokeWidth += 1
				w.SetRectSprite(rect, sprite)
			},
		},
		{
			name: "velocity",
			mutate: func(w *scene.World, circle, _ scene.EntityID) {
				w.SetVelocity(circle, scene.Velocity{Linear: math2d.Vec2{X: 99, Y: 0}})
			},
		},
		{
			name: "acceleration",
			mutate: func(w *scene.World, circle, _ scene.EntityID) {
				w.SetAcceleration(circle, scene.Acceleration{Linear: math2d.Vec2{X: 0, Y: 99}})
			},
		},
		{
			name: "damping",
			mutate: func(w *scene.World, circle, _ scene.EntityID) {
				w.SetDamping(circle, scene.Damping{Linear: 0.99})
			},
		},
		{
			name: "body.Restitution",
			mutate: func(w *scene.World, circle, _ scene.EntityID) {
				body, _ := w.Body(circle)
				body.Restitution = 0.99
				w.SetBody(circle, body)
			},
		},
		{
			name: "circleCollider.Radius",
			mutate: func(w *scene.World, circle, _ scene.EntityID) {
				w.SetCircleCollider(circle, scene.CircleCollider{Radius: 99})
			},
		},
		{
			name: "rectCollider.HalfExtends",
			mutate: func(w *scene.World, _, rect scene.EntityID) {
				w.SetRectCollider(rect, scene.RectCollider{HalfExtends: math2d.Vec2{X: 99, Y: 99}})
			},
		},
		{
			name: "remove component (Body)",
			mutate: func(w *scene.World, circle, _ scene.EntityID) {
				w.RemoveBody(circle)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := scene.NewWorld()
			circle, rect := buildChecksumFixture(w)

			before := w.Checksum()
			tc.mutate(w, circle, rect)

			if w.Checksum() == before {
				t.Errorf("%s: checksum did not change after mutation", tc.name)
			}
		})
	}
}

// TestChecksumDistinguishesAbsentFromZero guards design decision D2: each
// component map is hashed with a tag byte before its entries, so a component
// that is absent cannot collide with the same component present but holding
// its zero value.
func TestChecksumDistinguishesAbsentFromZero(t *testing.T) {
	withoutVelocity := scene.NewWorld()
	spawnAt(withoutVelocity, math2d.Vec2{X: 1, Y: 1})

	withZeroVelocity := scene.NewWorld()
	id := spawnAt(withZeroVelocity, math2d.Vec2{X: 1, Y: 1})
	withZeroVelocity.SetVelocity(id, scene.Velocity{})

	if withoutVelocity.Checksum() == withZeroVelocity.Checksum() {
		t.Fatal("checksum did not distinguish an absent Velocity from a present, zero-valued one")
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
