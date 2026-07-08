package scene

import "testing"

func TestWorldEntityCountCacheInvalidatesOnComponentChanges(t *testing.T) {
	w := NewWorld()
	first := w.Spawn()
	second := w.Spawn()

	w.SetTransform(first, Transform{})
	if got := w.entityCount(); got != 1 {
		t.Fatalf("entityCount after first transform = %d, want 1", got)
	}
	if got := w.entityCount(); got != 1 {
		t.Fatalf("cached entityCount = %d, want 1", got)
	}

	w.SetVelocity(first, Velocity{})
	if got := w.entityCount(); got != 1 {
		t.Fatalf("entityCount after second component on same entity = %d, want 1", got)
	}

	w.SetVelocity(second, Velocity{})
	if got := w.entityCount(); got != 2 {
		t.Fatalf("entityCount after second entity = %d, want 2", got)
	}

	w.RemoveVelocity(second)
	if got := w.entityCount(); got != 1 {
		t.Fatalf("entityCount after removing only component from second entity = %d, want 1", got)
	}

	w.Despawn(first)
	if got := w.entityCount(); got != 0 {
		t.Fatalf("entityCount after despawn = %d, want 0", got)
	}
}
