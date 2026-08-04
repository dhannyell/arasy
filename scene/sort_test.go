package scene_test

import (
	"testing"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
	"github.com/dhannyell/arasy/scene"
)

// spawnSortable gives every sprite the same radius, so the only thing telling
// two of them apart in the depth order is where they sit. The X coordinate is
// unused by a Y-axis sort and doubles as a name.
func spawnSortable(t *testing.T, w *scene.World, marker float64, layer int, y float64) scene.EntityID {
	t.Helper()
	id := w.Spawn()
	w.SetTransform(id, scene.Transform{
		Position: math2d.Vec2{X: marker, Y: y},
		Scale:    math2d.Vec2{X: 1, Y: 1},
	})
	w.SetCircleSprite(id, scene.CircleSprite{
		Radius:     2,
		BaseSprite: scene.BaseSprite{Layer: layer, Color: render.RGBA(1, 1, 1, 1)},
	})
	return id
}

// drawOrder reads the markers back out of the emitted commands, which is the
// only place the depth order is observable from outside the package.
func drawOrder(t *testing.T, commands []render.Command) []float64 {
	t.Helper()
	markers := make([]float64, 0, len(commands))
	for i, cmd := range commands {
		circle, ok := cmd.(render.DrawCircle)
		if !ok {
			t.Fatalf("command %d is %T, want render.DrawCircle", i, cmd)
		}
		markers = append(markers, circle.Center.X)
	}
	return markers
}

func expectOrder(t *testing.T, got, want []float64) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %d commands, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("draw order = %v, want %v", got, want)
		}
	}
}

// The depth order decides painter order in every frame, so it is pinned here
// before the axis becomes something a game can declare: a world that never
// mentions an axis has to keep coming out exactly like this.
func TestDepthSortOrdersByLayerThenDepthThenEntityID(t *testing.T) {
	world := scene.NewWorld()
	spawnSortable(t, world, 0, 1, 30)
	spawnSortable(t, world, 1, 0, 50)
	spawnSortable(t, world, 2, 1, 10)
	spawnSortable(t, world, 3, 1, 10) // ties with 2; EntityID breaks it
	spawnSortable(t, world, 4, 0, -5)

	expectOrder(t, drawOrder(t, world.RenderCommands()), []float64{4, 1, 2, 3, 0})
}

// Two runs of the same scene must paint in the same order. Without a total
// order a frame is only reproducible by luck, and a recorded replay is not
// reproducible at all.
func TestDepthSortIsDeterministic(t *testing.T) {
	build := func() []float64 {
		world := scene.NewWorld()
		for i, y := range []float64{7, 7, 7, 2, 9, 2} {
			spawnSortable(t, world, float64(i), 0, y)
		}
		return drawOrder(t, world.RenderCommands())
	}

	expectOrder(t, build(), build())
}

// The declared axis has to reproduce the pinned order above exactly, not
// approximately: a game that never mentions an axis is not opting into
// anything, and neither is one that spells out the default.
func TestDefaultSortAxisIsPlainY(t *testing.T) {
	build := func(axis math2d.Vec2) []float64 {
		world := scene.NewWorld()
		world.SetSortAxis(axis)
		spawnSortable(t, world, 0, 1, 30)
		spawnSortable(t, world, 1, 0, 50)
		spawnSortable(t, world, 2, 1, 10)
		spawnSortable(t, world, 3, 1, 10)
		spawnSortable(t, world, 4, 0, -5)
		return drawOrder(t, world.RenderCommands())
	}

	pinned := []float64{4, 1, 2, 3, 0}
	expectOrder(t, build(math2d.Vec2{}), pinned)
	expectOrder(t, build(math2d.Vec2{Y: 1}), pinned)
	// A direction, not a distance: scaling it must not reorder anything.
	expectOrder(t, build(math2d.Vec2{Y: 7}), pinned)
}

// The point of declaring an axis is that a game whose depth does not run down
// the screen can say so.
func TestSortAxisChoosesWhatCountsAsBehind(t *testing.T) {
	build := func(axis math2d.Vec2) []float64 {
		world := scene.NewWorld()
		world.SetSortAxis(axis)
		// Markers double as X here, so the two axes disagree by construction.
		spawnSortable(t, world, 30, 0, 10)
		spawnSortable(t, world, 10, 0, 30)
		spawnSortable(t, world, 20, 0, 20)
		return drawOrder(t, world.RenderCommands())
	}

	expectOrder(t, build(math2d.Vec2{X: 1}), []float64{10, 20, 30})
	expectOrder(t, build(math2d.Vec2{Y: 1}), []float64{30, 20, 10})
	// Negating the axis reverses depth, which is how a game whose world
	// coordinates run the other way says so.
	expectOrder(t, build(math2d.Vec2{X: -1}), []float64{30, 20, 10})
}

// A tilted camera is the case the axis exists for: depth stops being Y alone,
// and the sprites in between are the ones that move.
func TestDiagonalSortAxisOrdersByProjection(t *testing.T) {
	world := scene.NewWorld()
	world.SetSortAxis(math2d.Vec2{X: 1, Y: 1})
	spawnSortable(t, world, 0, 0, 10)  // projects to 10
	spawnSortable(t, world, 12, 0, 0)  // projects to 12
	spawnSortable(t, world, 3, 0, 3)   // projects to 6
	spawnSortable(t, world, 20, 0, -5) // projects to 15

	expectOrder(t, drawOrder(t, world.RenderCommands()), []float64{3, 0, 12, 20})
}

// The axis is a way of drawing the world, not a fact about it. If it reached
// the checksum, two hosts rendering the same simulation differently would be
// reported as a desync.
func TestSortAxisStaysOutOfChecksum(t *testing.T) {
	world := scene.NewWorld()
	spawnSortable(t, world, 0, 0, 10)
	spawnSortable(t, world, 1, 0, 20)
	before := world.Checksum()

	world.SetSortAxis(math2d.Vec2{X: 1, Y: -0.26})

	if after := world.Checksum(); after != before {
		t.Fatalf("checksum moved from %v to %v; the sort axis is presentation", before, after)
	}
}

// An entity can carry a sprite and no Transform; it has nowhere to be drawn,
// and it must not take the rest of the frame down with it.
func TestDepthSortToleratesSpriteWithoutTransform(t *testing.T) {
	world := scene.NewWorld()
	spawnSortable(t, world, 0, 0, 20)
	orphan := world.Spawn()
	world.SetCircleSprite(orphan, scene.CircleSprite{Radius: 2})
	spawnSortable(t, world, 2, 0, 10)

	expectOrder(t, drawOrder(t, world.RenderCommands()), []float64{2, 0})
}
