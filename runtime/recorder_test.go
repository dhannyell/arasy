package runtime

import (
	"testing"

	"github.com/dhannyell/frameforge/scene"
)

func TestRecorderCapturesOnlyKeyframeTicks(t *testing.T) {
	rec := newRecorder(12, 3)

	for tick := uint64(0); tick < 10; tick++ {
		rec.Capture(testSnapshot(tick))
	}

	if got, want := rec.Len(), 4; got != want {
		t.Fatalf("len = %d, want %d", got, want)
	}
	start, end, ok := rec.Range()
	if !ok || start != 0 || end != 9 {
		t.Fatalf("range = %d-%d ok=%v, want 0-9 ok=true", start, end, ok)
	}
	if snap, ok := rec.KeyframeAtOrBefore(8); !ok || snap.tick != 6 {
		t.Fatalf("keyframe before 8 = tick %d ok=%v, want 6 ok=true", snap.tick, ok)
	}
	if _, ok := rec.KeyframeAt(8); ok {
		t.Fatal("unexpected exact keyframe at tick 8")
	}
	if snap, ok := rec.KeyframeAt(9); !ok || snap.tick != 9 {
		t.Fatalf("keyframe at 9 = tick %d ok=%v, want 9 ok=true", snap.tick, ok)
	}
}

func TestRecorderRetainsHistoryWindowByKeyframeCount(t *testing.T) {
	rec := newRecorder(6, 2)

	for tick := uint64(0); tick <= 8; tick++ {
		rec.Capture(testSnapshot(tick))
	}

	if got, want := rec.Len(), 3; got != want {
		t.Fatalf("len = %d, want %d", got, want)
	}
	start, end, ok := rec.Range()
	if !ok || start != 4 || end != 8 {
		t.Fatalf("range = %d-%d ok=%v, want 4-8 ok=true", start, end, ok)
	}
	if _, ok := rec.KeyframeAtOrBefore(3); ok {
		t.Fatal("unexpected keyframe before retained window")
	}
}

func TestRecorderDropAfterRemovesAbandonedFuture(t *testing.T) {
	rec := newRecorder(12, 2)
	for tick := uint64(0); tick <= 10; tick++ {
		rec.Capture(testSnapshot(tick))
	}

	rec.DropAfter(5)

	start, end, ok := rec.Range()
	if !ok || start != 0 || end != 4 {
		t.Fatalf("range after drop = %d-%d ok=%v, want 0-4 ok=true", start, end, ok)
	}
	if _, ok := rec.KeyframeAtOrBefore(10); !ok {
		t.Fatal("expected retained keyframe at or before 10")
	}
	if snap, _ := rec.KeyframeAtOrBefore(10); snap.tick != 4 {
		t.Fatalf("keyframe before 10 after drop = %d, want 4", snap.tick)
	}
}

func TestRecorderIntervalOneKeepsDenseHistory(t *testing.T) {
	rec := newRecorder(4, 1)
	for tick := uint64(0); tick < 6; tick++ {
		rec.Capture(testSnapshot(tick))
	}

	if got, want := rec.Len(), 4; got != want {
		t.Fatalf("len = %d, want %d", got, want)
	}
	start, end, ok := rec.Range()
	if !ok || start != 2 || end != 5 {
		t.Fatalf("range = %d-%d ok=%v, want 2-5 ok=true", start, end, ok)
	}
}

func testSnapshot(tick uint64) driverSnapshot {
	return driverSnapshot{
		world: scene.NewWorld(),
		input: newDriverInput(nil).Snapshot(),
		tick:  tick,
	}
}
