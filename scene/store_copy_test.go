package scene

import (
	"slices"
	"testing"

	"github.com/dhannyell/frameforge/math2d"
)

func TestStoreCopyFromMatchesCloneAfterRemoveAndReadd(t *testing.T) {
	var src store[int]
	src.set(3, 30)
	src.set(1, 10)
	src.set(2, 20)
	src.remove(2)
	src.set(2, 200)

	want := src.clone()
	var got store[int]
	got.set(99, 99)
	got.copyFrom(&src)

	if len(got.sparse) != 0 {
		t.Fatalf("copyFrom sparse length = %d, want 0 like clone", len(got.sparse))
	}
	if !slices.Equal(got.ids, want.ids) || !slices.Equal(got.dense, want.dense) {
		t.Fatalf("copyFrom data = ids %v dense %v, want ids %v dense %v", got.ids, got.dense, want.ids, want.dense)
	}

	for _, id := range []EntityID{1, 2, 3, 4} {
		gotValue, gotOK := got.get(id)
		wantValue, wantOK := want.get(id)
		if gotValue != wantValue || gotOK != wantOK {
			t.Fatalf("get(%d) = (%d, %v), want (%d, %v)", id, gotValue, gotOK, wantValue, wantOK)
		}
	}
}

func TestSyncRenderStateReusesStoreCapacity(t *testing.T) {
	w := NewWorld()
	for range 32 {
		id := w.Spawn()
		w.SetTransform(id, Transform{Scale: math2d.Vec2{X: 1, Y: 1}})
	}
	w.SyncRenderState()

	if allocs := testing.AllocsPerRun(100, w.SyncRenderState); allocs != 0 {
		t.Fatalf("SyncRenderState allocs/run = %v, want 0", allocs)
	}
}
