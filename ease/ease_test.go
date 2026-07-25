package ease_test

import (
	"testing"

	"github.com/dhannyell/arasy/ease"
)

var allKinds = []ease.Kind{
	ease.Linear,
	ease.InQuad, ease.OutQuad, ease.InOutQuad,
	ease.InCubic, ease.OutCubic, ease.InOutCubic,
	ease.InQuart, ease.OutQuart, ease.InOutQuart,
	ease.InOutSine,
	ease.OutExpo,
	ease.OutBack,
	ease.OutElastic,
	ease.OutBounce,
}

func TestEaseEndpoints(t *testing.T) {
	for _, k := range allKinds {
		if got := ease.Apply(k, 0); got != 0 {
			t.Errorf("Apply(%v, 0) = %v, want 0", k, got)
		}
		if got := ease.Apply(k, 1); got != 1 {
			t.Errorf("Apply(%v, 1) = %v, want 1", k, got)
		}
	}
}

func TestProgressClamps(t *testing.T) {
	cases := []struct {
		elapsed, duration, want float64
	}{
		{0, 1, 0},
		{1, 1, 1},
		{0.5, 1, 0.5},
		{-1, 1, 0},
		{2, 1, 1},
		{1, 0, 1},
		{0, 0, 1},
		{0, -1, 1},
	}
	for _, c := range cases {
		if got := ease.Progress(c.elapsed, c.duration); got != c.want {
			t.Errorf("Progress(%v, %v) = %v, want %v", c.elapsed, c.duration, got, c.want)
		}
	}
}
