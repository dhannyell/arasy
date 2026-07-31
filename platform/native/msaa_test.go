//go:build windows && amd64

package native

import (
	"testing"
)

func TestNormalizeMSAASamples(t *testing.T) {
	tests := []struct {
		requested int
		want      uint32
	}{
		{0, defaultMSAASamples}, // unset keeps the engine default
		{1, 1},                  // explicit off
		{2, 2},
		{4, 4},
		{8, 8},
		{3, defaultMSAASamples}, // unsupported counts fall back, never fail
		{16, defaultMSAASamples},
		{-1, defaultMSAASamples},
	}
	for _, tt := range tests {
		if got := normalizeMSAASamples(tt.requested, nil); got != tt.want {
			t.Errorf("normalizeMSAASamples(%d) = %d, want %d", tt.requested, got, tt.want)
		}
	}
}
