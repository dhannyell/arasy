//go:build windows && amd64

package native

import "testing"

func TestGrowBufferCapKeepsHeadroomAfterGrowth(t *testing.T) {
	const kib = 1024
	if got := growBufferCap(64*kib, 128664); got != 256*kib {
		t.Fatalf("growBufferCap(64 KiB, 128664) = %d, want 256 KiB", got)
	}
	if got := growBufferCap(256*kib, 139320); got != 256*kib {
		t.Fatalf("existing 256 KiB capacity changed to %d", got)
	}
}
