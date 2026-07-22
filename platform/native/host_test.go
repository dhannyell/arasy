//go:build windows && amd64

package native

import (
	"testing"
	"time"
)

func TestFrameBudget(t *testing.T) {
	tests := []struct {
		fps  int
		want time.Duration
	}{
		{60, time.Second / 60},
		{120, time.Second / 120},
		{0, 0},
		{-1, 0},
	}
	for _, tt := range tests {
		got := frameBudget(tt.fps)
		if got != tt.want {
			t.Errorf("frameBudget(%d) = %v; want %v", tt.fps, got, tt.want)
		}
	}
}
