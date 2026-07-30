package gpubuffer_test

import (
	"testing"

	gpubuffer "github.com/dhannyell/arasy/internal/render/gpu/buffer"
)

func TestGrowCapacityKeepsHeadroomAfterGrowth(t *testing.T) {
	const kib = 1024

	if got := gpubuffer.GrowCapacity(64*kib, 128664); got != 256*kib {
		t.Fatalf("GrowCapacity(64 KiB, 128664) = %d, want 256 KiB", got)
	}
	if got := gpubuffer.GrowCapacity(256*kib, 139320); got != 256*kib {
		t.Fatalf("existing 256 KiB capacity changed to %d", got)
	}
}

func TestGrowCapacityUsesSharedInitialCapacity(t *testing.T) {
	if got := gpubuffer.GrowCapacity(0, 1); got != gpubuffer.InitialCapacity {
		t.Fatalf("GrowCapacity(0, 1) = %d, want %d", got, gpubuffer.InitialCapacity)
	}
}
