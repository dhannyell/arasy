package scene

import (
	"fmt"
	"testing"
)

func assertInvariant[T any](t *testing.T, s *store[T]) {
	t.Helper()
	if len(s.ids) != len(s.dense) {
		t.Fatalf("ids/dense length mismatch: ids=%d dense=%d", len(s.ids), len(s.dense))
	}
	for i, id := range s.ids {
		if int(s.sparse[int(id)]) != i {
			t.Fatalf("sparse[ids[%d]=%d] = %d, want %d", i, id, s.sparse[int(id)], i)
		}
	}
	for i := 0; i < len(s.sparse); i++ {
		idx := s.sparse[i]
		if idx == noIndex {
			continue
		}
		if int(idx) >= len(s.ids) {
			t.Fatalf("sparse[%d] = %d out of range (len(ids)=%d)", i, idx, len(s.ids))
		}
		if s.ids[int(idx)] != EntityID(i) {
			t.Fatalf("ids[sparse[%d]=%d] = %d, want %d", i, idx, s.ids[int(idx)], i)
		}
	}
}

func TestGet(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(*store[int])
		id      EntityID
		wantVal int
		wantOk  bool
	}{
		{"empty store", func(s *store[int]) {}, 0, 0, false},
		{"non-existent id", func(s *store[int]) { s.set(5, 50) }, 3, 0, false},
		{"existing id", func(s *store[int]) { s.set(1, 10); s.set(2, 20) }, 2, 20, true},
		{"after remove", func(s *store[int]) { s.set(1, 10); s.remove(1) }, 1, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var s store[int]
			tt.setup(&s)
			v, ok := s.get(tt.id)
			if ok != tt.wantOk {
				t.Fatalf("get(%d) ok=%v, want %v", tt.id, ok, tt.wantOk)
			}
			if v != tt.wantVal {
				t.Fatalf("get(%d) = %d, want %d", tt.id, v, tt.wantVal)
			}
		})
	}
}

func TestSet(t *testing.T) {
	tests := []struct {
		name    string
		ops     func(*store[int])
		wantLen int
		wantGet []struct {
			id  EntityID
			val int
		}
		wantIds []EntityID
	}{
		{
			name:    "empty store",
			ops:     func(s *store[int]) { s.set(0, 42) },
			wantLen: 1,
			wantGet: []struct {
				id  EntityID
				val int
			}{{0, 42}},
		},
		{
			name:    "monotonic increasing",
			ops:     func(s *store[int]) { s.set(1, 10); s.set(2, 20); s.set(3, 30) },
			wantLen: 3,
			wantGet: []struct {
				id  EntityID
				val int
			}{{1, 10}, {2, 20}, {3, 30}},
		},
		{
			name:    "out of order",
			ops:     func(s *store[int]) { s.set(3, 30); s.set(1, 10); s.set(2, 20) },
			wantLen: 3,
			wantGet: []struct {
				id  EntityID
				val int
			}{{1, 10}, {2, 20}, {3, 30}},
			wantIds: []EntityID{1, 2, 3},
		},
		{
			name:    "update existing",
			ops:     func(s *store[int]) { s.set(1, 10); s.set(2, 20); s.set(1, 99) },
			wantLen: 2,
			wantGet: []struct {
				id  EntityID
				val int
			}{{1, 99}},
		},
		{
			name: "reinsert after remove middle",
			ops: func(s *store[int]) {
				s.set(1, 10)
				s.set(2, 20)
				s.set(3, 30)
				s.remove(2)
				s.set(2, 200)
			},
			wantLen: 3,
			wantGet: []struct {
				id  EntityID
				val int
			}{{2, 200}},
			wantIds: []EntityID{1, 2, 3},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var s store[int]
			tt.ops(&s)
			assertInvariant(t, &s)
			if s.len() != tt.wantLen {
				t.Fatalf("len=%d, want %d", s.len(), tt.wantLen)
			}
			for _, g := range tt.wantGet {
				v, ok := s.get(g.id)
				if !ok {
					t.Fatalf("get(%d) ok=false, want true", g.id)
				}
				if v != g.val {
					t.Fatalf("get(%d) = %d, want %d", g.id, v, g.val)
				}
			}
			if tt.wantIds != nil {
				for i, want := range tt.wantIds {
					if s.ids[i] != want {
						t.Fatalf("ids[%d] = %d, want %d", i, s.ids[i], want)
					}
				}
			}
		})
	}
}

func TestRemove(t *testing.T) {
	type removeCase struct {
		name    string
		setup   func(*store[int])
		remove  EntityID
		wantLen int
	}

	noOpCases := []removeCase{
		{"non-existent id",
			func(s *store[int]) { s.set(1, 10) },
			99, 1},
		{"out of bounds id",
			func(s *store[int]) { s.set(1, 10) },
			500, 1},
	}
	for _, tt := range noOpCases {
		t.Run("no-op: "+tt.name, func(t *testing.T) {
			var s store[int]
			tt.setup(&s)
			s.remove(tt.remove)
			assertInvariant(t, &s)
			if s.len() != tt.wantLen {
				t.Fatalf("len=%d, want %d", s.len(), tt.wantLen)
			}
		})
	}

	threeSetup := func(s *store[int]) { s.set(1, 10); s.set(2, 20); s.set(3, 30) }

	validCases := []removeCase{
		{"first", threeSetup, 1, 2},
		{"middle", threeSetup, 2, 2},
		{"last", threeSetup, 3, 2},
	}
	for _, tt := range validCases {
		t.Run("valid: "+tt.name, func(t *testing.T) {
			var s store[int]
			tt.setup(&s)
			s.remove(tt.remove)
			assertInvariant(t, &s)
			if s.len() != tt.wantLen {
				t.Fatalf("len=%d, want %d", s.len(), tt.wantLen)
			}
			if s.has(tt.remove) {
				t.Fatalf("has(%d) = true after remove, want false", tt.remove)
			}
		})
	}
}

func TestRemoveAllOneByOne(t *testing.T) {
	var s store[int]
	s.set(1, 10)
	s.set(2, 20)
	s.set(3, 30)
	s.remove(1)
	s.remove(2)
	s.remove(3)
	assertInvariant(t, &s)
	if s.len() != 0 {
		t.Fatalf("len=%d, want 0", s.len())
	}
	if s.has(1) || s.has(2) || s.has(3) {
		t.Fatalf("has returned true after removing all")
	}
}

func TestHas(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*store[int])
		id    EntityID
		want  bool
	}{
		{"empty store", func(s *store[int]) {}, 0, false},
		{"after set", func(s *store[int]) { s.set(5, 50) }, 5, true},
		{"after remove", func(s *store[int]) { s.set(5, 50); s.remove(5) }, 5, false},
		{"out of range", func(s *store[int]) { s.set(1, 10) }, 1000, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var s store[int]
			tt.setup(&s)
			if got := s.has(tt.id); got != tt.want {
				t.Fatalf("has(%d) = %v, want %v", tt.id, got, tt.want)
			}
		})
	}
}

func TestLen(t *testing.T) {
	tests := []struct {
		name    string
		ops     func(*store[int])
		wantLen int
	}{
		{"empty store", func(s *store[int]) {}, 0},
		{"after 1 insert", func(s *store[int]) { s.set(1, 10) }, 1},
		{"after 2 inserts", func(s *store[int]) { s.set(1, 10); s.set(2, 20) }, 2},
		{"after 3 inserts", func(s *store[int]) { s.set(1, 10); s.set(2, 20); s.set(3, 30) }, 3},
		{"after 1 remove of 3", func(s *store[int]) {
			s.set(1, 10)
			s.set(2, 20)
			s.set(3, 30)
			s.remove(2)
		}, 2},
		{"after 2 removes of 3", func(s *store[int]) {
			s.set(1, 10)
			s.set(2, 20)
			s.set(3, 30)
			s.remove(2)
			s.remove(1)
		}, 1},
		{"unchanged after update", func(s *store[int]) {
			s.set(1, 10)
			s.set(2, 20)
			s.set(1, 99)
		}, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var s store[int]
			tt.ops(&s)
			if s.len() != tt.wantLen {
				t.Fatalf("len=%d, want %d", s.len(), tt.wantLen)
			}
		})
	}
}

func TestCloneIndependence(t *testing.T) {
	var s store[int]
	s.set(1, 10)
	s.set(2, 20)
	clone := s.clone()

	clone.set(3, 30)
	clone.remove(1)

	if s.len() != 2 {
		t.Fatalf("original len changed to %d after clone mutation, want 2", s.len())
	}
	if !s.has(1) {
		t.Fatalf("original lost id=1 after clone mutation")
	}
	if s.has(3) {
		t.Fatalf("original gained id=3 from clone mutation")
	}
}

func TestCloneEmptyStore(t *testing.T) {
	var s store[int]
	clone := s.clone()
	if clone.len() != 0 {
		t.Fatalf("clone of empty store has len=%d, want 0", clone.len())
	}
}

func TestCloneValues(t *testing.T) {
	var s store[int]
	s.set(1, 10)
	s.set(2, 20)
	clone := s.clone()

	for _, want := range []struct {
		id  EntityID
		val int
	}{
		{1, 10}, {2, 20},
	} {
		v, ok := clone.get(want.id)
		if !ok || v != want.val {
			t.Fatalf("clone get(%d) = (%d, %v), want (%d, true)", want.id, v, ok, want.val)
		}
	}
}

func TestCloneDoesNotCopySparseIndex(t *testing.T) {
	var s store[int]
	s.set(1000, 10)
	clone := s.clone()

	if len(clone.sparse) != 0 {
		t.Fatalf("clone sparse len = %d, want 0 before first lookup", len(clone.sparse))
	}

	v, ok := clone.get(1000)
	if !ok || v != 10 {
		t.Fatalf("clone get(1000) = (%d, %v), want (10, true)", v, ok)
	}
	assertInvariant(t, &clone)
}

func TestCloneLazySparseSupportsMutation(t *testing.T) {
	var s store[int]
	s.set(100, 10)
	s.set(200, 20)
	clone := s.clone()

	clone.remove(100)
	clone.set(150, 15)
	clone.set(200, 200)

	assertInvariant(t, &clone)

	if clone.has(100) {
		t.Fatalf("clone still has removed id 100")
	}
	for _, want := range []struct {
		id  EntityID
		val int
	}{
		{150, 15},
		{200, 200},
	} {
		got, ok := clone.get(want.id)
		if !ok || got != want.val {
			t.Fatalf("clone get(%d) = (%d, %v), want (%d, true)", want.id, got, ok, want.val)
		}
	}
}

func TestSetExceedsMaxStoreIndexPanics(t *testing.T) {
	var s store[int]
	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("expected panic for id > maxStoreIndex, got none")
		}
		msg := fmt.Sprint(r)
		if msg != "scene store EntityID exceeds int32 sparse index capacity" {
			t.Fatalf("unexpected panic message: %v", msg)
		}
	}()
	s.set(maxStoreIndex+1, 42)
}

func TestInterleavedSetRemove(t *testing.T) {
	var s store[int]
	s.set(1, 10)
	s.set(2, 20)
	s.remove(1)
	s.set(3, 30)
	s.set(2, 200)
	s.remove(3)
	s.set(1, 100)
	assertInvariant(t, &s)

	if s.len() != 2 {
		t.Fatalf("len=%d, want 2", s.len())
	}
	for _, want := range []struct {
		id  EntityID
		val int
	}{
		{1, 100}, {2, 200},
	} {
		v, ok := s.get(want.id)
		if !ok {
			t.Fatalf("get(%d) ok=false, want true", want.id)
		}
		if v != want.val {
			t.Fatalf("get(%d) = %d, want %d", want.id, v, want.val)
		}
	}
}

func TestInterleavedStressManyOperations(t *testing.T) {
	var s store[int]

	for i := EntityID(0); i < 100; i++ {
		s.set(i, int(i*10))
	}
	assertInvariant(t, &s)
	if s.len() != 100 {
		t.Fatalf("len=%d after 100 inserts, want 100", s.len())
	}

	for i := EntityID(0); i < 50; i++ {
		s.remove(i)
	}
	assertInvariant(t, &s)
	if s.len() != 50 {
		t.Fatalf("len=%d after 50 removes, want 50", s.len())
	}

	for i := EntityID(0); i < 50; i++ {
		s.set(i, int(i*100))
	}
	assertInvariant(t, &s)
	if s.len() != 100 {
		t.Fatalf("len=%d after re-inserting 50, want 100", s.len())
	}

	for i := EntityID(0); i < 100; i++ {
		v, ok := s.get(i)
		if !ok {
			t.Fatalf("get(%d) ok=false after re-insert", i)
		}
		if i < 50 {
			if v != int(i*100) {
				t.Fatalf("get(%d) = %d, want %d", i, v, i*100)
			}
		} else {
			if v != int(i*10) {
				t.Fatalf("get(%d) = %d, want %d", i, v, i*10)
			}
		}
	}

	for i := EntityID(0); i < 100; i++ {
		s.remove(i)
	}
	assertInvariant(t, &s)
	if s.len() != 0 {
		t.Fatalf("len=%d after removing all, want 0", s.len())
	}
}
