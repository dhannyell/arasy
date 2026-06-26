package debug_test

import (
	"testing"

	"github.com/dhannyell/frameforge/debug"
	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/scene"
)

func TestRecorderStartsEmpty(t *testing.T) {
	recorder := debug.NewRecorder(1)

	if recorder.Len() != 0 {
		t.Fatalf("Len() = %v, want 0", recorder.Len())
	}
}

func TestRecorderCaptureStoresSnapshotClone(t *testing.T) {
	world, entity := newWorldWithPosition(math2d.Vec2{X: 10, Y: 20})
	recorder := debug.NewRecorder(4)

	recorder.Capture(world)

	world.SetTransform(entity, scene.Transform{
		Position: math2d.Vec2{X: 99, Y: 88},
		Scale:    math2d.Vec2{X: 1, Y: 1},
	})

	snapshot, ok := recorder.Back()
	if !ok {
		t.Fatal("Back() ok = false, want true")
	}

	transform, ok := snapshot.Transform(entity)
	if !ok {
		t.Fatal("expected snapshot to keep captured transform")
	}

	want := math2d.Vec2{X: 10, Y: 20}
	if transform.Position != want {
		t.Fatalf("snapshot position = %+v, want %+v", transform.Position, want)
	}
}

func TestRecorderBackReturnsLatestSnapshotAndRemovesIt(t *testing.T) {
	firstWorld, _ := newWorldWithPosition(math2d.Vec2{X: 1, Y: 1})
	secondWorld, secondEntity := newWorldWithPosition(math2d.Vec2{X: 2, Y: 2})

	recorder := debug.NewRecorder(4)
	recorder.Capture(firstWorld)
	recorder.Capture(secondWorld)

	snapshot, ok := recorder.Back()
	if !ok {
		t.Fatal("Back() ok = false, want true")
	}

	if recorder.Len() != 1 {
		t.Fatalf("Len() after Back() = %v, want 1", recorder.Len())
	}

	transform, ok := snapshot.Transform(secondEntity)
	if !ok {
		t.Fatal("expected latest snapshot to contain second entity")
	}

	want := math2d.Vec2{X: 2, Y: 2}
	if transform.Position != want {
		t.Fatalf("latest snapshot position = %+v, want %+v", transform.Position, want)
	}
}

func TestRecorderBackReturnsFalseWhenEmpty(t *testing.T) {
	recorder := debug.NewRecorder(1)

	world, ok := recorder.Back()
	if ok {
		t.Fatal("Back() ok = true, want false")
	}

	if world != nil {
		t.Fatalf("Back() world = %+v, want nil", world)
	}
}

func TestRecorderClearRemovesSnapshots(t *testing.T) {
	world, _ := newWorldWithPosition(math2d.Vec2{X: 1, Y: 1})
	recorder := debug.NewRecorder(2)

	recorder.Capture(world)
	recorder.Clear()

	if recorder.Len() != 0 {
		t.Fatalf("Len() after Clear() = %v, want 0", recorder.Len())
	}

	if snapshot, ok := recorder.Back(); ok || snapshot != nil {
		t.Fatalf("Back() after Clear() = (%+v, %v), want (nil, false)", snapshot, ok)
	}
}

func TestRecorderRespectsLimit(t *testing.T) {
	firstWorld, _ := newWorldWithPosition(math2d.Vec2{X: 1, Y: 1})
	secondWorld, secondEntity := newWorldWithPosition(math2d.Vec2{X: 2, Y: 2})
	thirdWorld, thirdEntity := newWorldWithPosition(math2d.Vec2{X: 3, Y: 3})

	recorder := debug.NewRecorder(2)
	recorder.Capture(firstWorld)
	recorder.Capture(secondWorld)
	recorder.Capture(thirdWorld)

	if recorder.Len() != 2 {
		t.Fatalf("Len() = %v, want 2", recorder.Len())
	}

	latest, ok := recorder.Back()
	if !ok {
		t.Fatal("Back() latest ok = false, want true")
	}
	latestTransform, ok := latest.Transform(thirdEntity)
	if !ok {
		t.Fatal("expected latest snapshot to contain third entity")
	}
	if latestTransform.Position != (math2d.Vec2{X: 3, Y: 3}) {
		t.Fatalf("latest snapshot position = %+v, want (3, 3)", latestTransform.Position)
	}

	previous, ok := recorder.Back()
	if !ok {
		t.Fatal("Back() previous ok = false, want true")
	}
	previousTransform, ok := previous.Transform(secondEntity)
	if !ok {
		t.Fatal("expected previous snapshot to contain second entity")
	}
	if previousTransform.Position != (math2d.Vec2{X: 2, Y: 2}) {
		t.Fatalf("previous snapshot position = %+v, want (2, 2)", previousTransform.Position)
	}

	if snapshot, ok := recorder.Back(); ok || snapshot != nil {
		t.Fatalf("Back() after limited snapshots = (%+v, %v), want (nil, false)", snapshot, ok)
	}
}

func TestRecorderDoesNotCaptureWhenLimitIsZeroOrNegative(t *testing.T) {
	world, _ := newWorldWithPosition(math2d.Vec2{X: 1, Y: 1})

	for _, limit := range []int{0, -1} {
		recorder := debug.NewRecorder(limit)
		recorder.Capture(world)

		if recorder.Len() != 0 {
			t.Fatalf("Len() with limit %v = %v, want 0", limit, recorder.Len())
		}
	}
}

func TestRecorderDoesNotCaptureNilWorld(t *testing.T) {
	recorder := debug.NewRecorder(1)

	recorder.Capture(nil)

	if recorder.Len() != 0 {
		t.Fatalf("Len() after nil Capture() = %v, want 0", recorder.Len())
	}
}

func newWorldWithPosition(position math2d.Vec2) (*scene.World, scene.EntityID) {
	world := scene.NewWorld()
	entity := world.Spawn()
	world.SetTransform(entity, scene.Transform{
		Position: position,
		Scale:    math2d.Vec2{X: 1, Y: 1},
	})

	return world, entity
}
