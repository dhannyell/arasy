package particle

import (
	"math/bits"
	"slices"
	"testing"

	"github.com/dhannyell/frameforge/math2d"
)

func TestActiveSlotsStayInSyncAcrossSpawnDeathAndReuse(t *testing.T) {
	decl := baseDecl()
	decl.Capacity = 4
	decl.BurstSize = 4
	decl.Lifetime = Range{Min: 0.2, Max: 0.2}

	emitter := NewEmitter(decl, 1)
	position := math2d.Vec2{}
	emitter.Step(position, 0, 0, 0, 0.01, decl.Capacity)
	emitter.Step(position, 0, 0, 1, 0.05, decl.Capacity)
	assertActiveSlotCount(t, emitter, 4)
	if got := len(emitter.Alive(nil)); got != 4 {
		t.Fatalf("Alive() after spawn = %d particles, want 4", got)
	}

	emitter.Step(position, 0, 0, 1, 0.2, decl.Capacity)
	assertActiveSlotCount(t, emitter, 0)
	if got := len(emitter.Alive(nil)); got != 0 {
		t.Fatalf("Alive() after death = %d particles, want 0", got)
	}

	emitter.Step(position, 0, 0, 2, 0.05, decl.Capacity)
	assertActiveSlotCount(t, emitter, 4)
	if got := len(emitter.Alive(nil)); got != 4 {
		t.Fatalf("Alive() after slot reuse = %d particles, want 4", got)
	}
}

func TestStepAndAliveVisitFragmentedActiveSlotsInStableOrder(t *testing.T) {
	decl := baseDecl()
	decl.Capacity = 130
	decl.Gravity = math2d.Vec2{}
	decl.Drag = 1

	emitter := NewEmitter(decl, 1)
	activeSlots := []int{2, 64, 129}
	for _, slot := range activeSlots {
		emitter.pool[slot] = Particle{
			Pos:       math2d.Vec2{X: float64(slot)},
			Vel:       math2d.Vec2{X: 1},
			Life:      100,
			SizeStart: 1,
			SizeEnd:   1,
		}
		emitter.markAlive(slot)
	}

	emitter.free = emitter.free[:0]
	for slot := len(emitter.pool) - 1; slot >= 0; slot-- {
		if !containsSlot(activeSlots, slot) {
			emitter.free = append(emitter.free, int32(slot))
		}
	}

	emitter.Step(math2d.Vec2{}, 0, 0, 0, 1, decl.Capacity)
	live := emitter.Alive(nil)
	if len(live) != len(activeSlots) {
		t.Fatalf("Alive() = %d particles, want %d", len(live), len(activeSlots))
	}
	for i, slot := range activeSlots {
		wantX := float64(slot + 1)
		if live[i].Pos.X != wantX {
			t.Fatalf("Alive()[%d].Pos.X = %v, want %v for slot %d", i, live[i].Pos.X, wantX, slot)
		}
	}
}

func assertActiveSlotCount(t *testing.T, emitter *Emitter, want int) {
	t.Helper()

	bitsCount := 0
	for _, word := range emitter.aliveBits {
		bitsCount += bits.OnesCount64(word)
	}
	if bitsCount != want {
		t.Fatalf("active bit count = %d, want %d", bitsCount, want)
	}
	if got := len(emitter.pool) - len(emitter.free); got != want {
		t.Fatalf("pool/free live count = %d, want %d", got, want)
	}
	if got := emitter.AliveCount(); got != want {
		t.Fatalf("AliveCount() = %d, want %d", got, want)
	}
}

func containsSlot(slots []int, want int) bool {
	return slices.Contains(slots, want)
}
