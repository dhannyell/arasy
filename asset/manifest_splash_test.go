package asset_test

import (
	"testing"
	"testing/fstest"

	"github.com/dhannyell/arasy/asset"
)

func TestLoadSplashEffectAnchorsDefaultOnlyWhenOmitted(t *testing.T) {
	layer := fstest.MapFS{
		"manifest.json": {Data: []byte(`{
			"version": 1,
			"textures": [{"name": "boot", "file": "boot.png"}],
			"effects": [{
				"name": "spark",
				"capacity": 1,
				"lifetime": [1, 1],
				"size_start": [1, 1]
			}],
			"splash": [
				{
					"texture": "boot",
					"cell_w": 1,
					"cell_h": 1,
					"columns": 1,
					"frames": 1,
					"effect": "spark"
				},
				{
					"texture": "boot",
					"cell_w": 1,
					"cell_h": 1,
					"columns": 1,
					"frames": 1,
					"effect": "spark",
					"effect_anchor_x": 0
				},
				{
					"texture": "boot",
					"cell_w": 1,
					"cell_h": 1,
					"columns": 1,
					"frames": 1,
					"effect": "spark",
					"effect_anchor_y": 0
				}
			]
		}`)},
		"boot.png": {Data: []byte("boot-texture")},
	}

	manifest, err := asset.Load(layer)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	got := manifest.Splash()
	want := [][2]float64{
		{0.5, 0.5},
		{0, 0.5},
		{0.5, 0},
	}
	if len(got) != len(want) {
		t.Fatalf("Splash() len = %d, want %d", len(got), len(want))
	}
	for i, anchors := range want {
		if got[i].EffectAnchorX != anchors[0] || got[i].EffectAnchorY != anchors[1] {
			t.Errorf(
				"Splash()[%d] anchors = (%g, %g), want (%g, %g)",
				i,
				got[i].EffectAnchorX,
				got[i].EffectAnchorY,
				anchors[0],
				anchors[1],
			)
		}
	}
}
