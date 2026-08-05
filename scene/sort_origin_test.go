package scene_test

import (
	"testing"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
	"github.com/dhannyell/arasy/scene"
)

// The failure this whole default exists to stop: a tall tree and a short bush
// planted on the same ground line. Their centres are at different depths and
// their bases are not, so sorting by centre draws the tree behind the bush it
// is standing next to.
func TestSpritesOnTheSameGroundLineSortByTheirBase(t *testing.T) {
	const groundY = 100

	world := scene.NewWorld()
	tree := world.Spawn()
	world.SetTransform(tree, scene.Transform{
		Position: math2d.Vec2{X: 0, Y: groundY - 40}, // 80 tall, so its centre is high up
		Scale:    math2d.Vec2{X: 1, Y: 1},
	})
	world.SetImageSprite(tree, scene.ImageSprite{
		Size: math2d.Vec2{X: 20, Y: 80},
	})

	bush := world.Spawn()
	world.SetTransform(bush, scene.Transform{
		Position: math2d.Vec2{X: 30, Y: groundY - 5}, // 10 tall, centre near the ground
		Scale:    math2d.Vec2{X: 1, Y: 1},
	})
	world.SetImageSprite(bush, scene.ImageSprite{
		Size: math2d.Vec2{X: 20, Y: 10},
	})

	commands := world.RenderCommands()
	if len(commands) != 2 {
		t.Fatalf("got %d commands, want 2", len(commands))
	}

	// Both bases sit on groundY, so EntityID decides and the tree, spawned
	// first, draws first. What must not happen is the bush drawing first
	// because the tree's centre is 35 units further back.
	first, ok := commands[0].(render.DrawImage)
	if !ok {
		t.Fatalf("command 0 is %T, want render.DrawImage", commands[0])
	}
	if first.Dest.Min.X != -10 {
		t.Errorf("the bush drew first: the tall sprite was sorted by its centre, not its base")
	}
}

func TestSortOffsetMovesTheSortingPoint(t *testing.T) {
	// Two identical sprites, the second nearer the camera by every measure.
	// Only an offset can put it behind the first.
	build := func(offset math2d.Vec2) []float64 {
		world := scene.NewWorld()
		spawnSortable(t, world, 0, 0, 10)

		near := world.Spawn()
		world.SetTransform(near, scene.Transform{
			Position: math2d.Vec2{X: 1, Y: 20},
			Scale:    math2d.Vec2{X: 1, Y: 1},
		})
		world.SetCircleSprite(near, scene.CircleSprite{
			Radius:     2,
			BaseSprite: scene.BaseSprite{SortOffset: offset},
		})

		return drawOrder(t, world.RenderCommands())
	}

	expectOrder(t, build(math2d.Vec2{}), []float64{0, 1})
	expectOrder(t, build(math2d.Vec2{Y: -15}), []float64{1, 0})
	// The offset is a point in the world, so a sideways one only matters to a
	// world whose depth runs sideways.
	expectOrder(t, build(math2d.Vec2{X: -50}), []float64{0, 1})
}

// Each sprite type has to find its own base, or the default is only half a
// rule and mixing sprite types reintroduces the bug it fixes.
func TestSortBaseIsDerivedPerSpriteType(t *testing.T) {
	const groundY = 50

	world := scene.NewWorld()

	circle := world.Spawn()
	world.SetTransform(circle, scene.Transform{
		Position: math2d.Vec2{X: 0, Y: groundY - 12},
		Scale:    math2d.Vec2{X: 1, Y: 1},
	})
	world.SetCircleSprite(circle, scene.CircleSprite{Radius: 12})

	rect := world.Spawn()
	world.SetTransform(rect, scene.Transform{
		Position: math2d.Vec2{X: 40, Y: groundY - 30},
		Scale:    math2d.Vec2{X: 1, Y: 1},
	})
	world.SetRectSprite(rect, scene.RectSprite{
		Rect: math2d.NewRectFromCorners(
			math2d.Vec2{X: -10, Y: -30},
			math2d.Vec2{X: 10, Y: 30},
		),
	})

	image := world.Spawn()
	world.SetTransform(image, scene.Transform{
		Position: math2d.Vec2{X: 80, Y: groundY - 3},
		Scale:    math2d.Vec2{X: 1, Y: 1},
	})
	world.SetImageSprite(image, scene.ImageSprite{Size: math2d.Vec2{X: 10, Y: 6}})

	// All three bases land on groundY despite three different heights and
	// three different ways of describing a shape, so EntityID orders them.
	commands := world.RenderCommands()
	if len(commands) != 3 {
		t.Fatalf("got %d commands, want 3", len(commands))
	}
	if _, ok := commands[0].(render.DrawCircle); !ok {
		t.Fatalf("command 0 is %T; the three bases did not agree on the ground line", commands[0])
	}
	if _, ok := commands[1].(render.DrawRect); !ok {
		t.Fatalf("command 1 is %T; the three bases did not agree on the ground line", commands[1])
	}
	if _, ok := commands[2].(render.DrawImage); !ok {
		t.Fatalf("command 2 is %T; the three bases did not agree on the ground line", commands[2])
	}
}

// A sprite sorts by where it stands, not by where it has been thrown. Sorting
// a ball by its lifted position would send it in front of a post it is
// passing behind, halfway through the arc.
func TestHeightStaysOutOfTheSortKey(t *testing.T) {
	build := func(z float64) []float64 {
		world := scene.NewWorld()
		world.SetProjection(render.Projection{LiftPerZ: 1})
		spawnSortable(t, world, 0, 0, 30)

		ball := world.Spawn()
		world.SetTransform(ball, scene.Transform{
			Position: math2d.Vec2{X: 1, Y: 40},
			Scale:    math2d.Vec2{X: 1, Y: 1},
		})
		world.SetCircleSprite(ball, scene.CircleSprite{Radius: 2})
		world.SetHeight(ball, scene.Height{Z: z})

		return drawOrder(t, world.RenderCommands())
	}

	expectOrder(t, build(0), []float64{0, 1})
	expectOrder(t, build(200), []float64{0, 1})
}

func TestSortOffsetStaysOutOfChecksum(t *testing.T) {
	world := scene.NewWorld()
	id := spawnSortable(t, world, 0, 0, 10)
	before := world.Checksum()

	sprite, _ := world.CircleSprite(id)
	sprite.SortOffset = math2d.Vec2{X: 3, Y: -9}
	world.SetCircleSprite(id, sprite)

	if after := world.Checksum(); after != before {
		t.Fatalf("checksum moved from %v to %v; the sort offset is presentation", before, after)
	}
}
