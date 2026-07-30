package asset_test

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"strings"
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
		"boot.png": {Data: splashTestPNG(t, 1, 1)},
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

func TestLoadSplashAcceptsAtlasWithinTexture(t *testing.T) {
	manifest, err := loadSplashEntry(t, 12, 8, `{
		"texture": "boot",
		"cell_w": 4,
		"cell_h": 4,
		"columns": 3,
		"frames": 6,
		"rate": 6,
		"effect_anchor_x": 0,
		"effect_anchor_y": 1,
		"effect_intensity": 0
	}`)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	splash := manifest.Splash()
	if len(splash) != 1 || splash[0].Frames != 6 || splash[0].Rate != 6 {
		t.Fatalf("Splash() = %#v, want one six-frame segment", splash)
	}
}

func TestLoadSplashAcceptsJPEGTextureDimensions(t *testing.T) {
	var texture bytes.Buffer
	if err := jpeg.Encode(&texture, image.NewNRGBA(image.Rect(0, 0, 12, 8)), nil); err != nil {
		t.Fatalf("jpeg.Encode() error = %v", err)
	}

	manifest, err := asset.Load(fstest.MapFS{
		"manifest.json": {Data: []byte(`{
			"version": 1,
			"textures": [{"name": "boot", "file": "boot.jpg"}],
			"splash": [{
				"texture": "boot",
				"cell_w": 4,
				"cell_h": 4,
				"columns": 3,
				"frames": 6
			}]
		}`)},
		"boot.jpg": {Data: texture.Bytes()},
	})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got := manifest.Splash(); len(got) != 1 || got[0].Frames != 6 {
		t.Fatalf("Splash() = %#v, want one six-frame segment", got)
	}
}

func TestLoadSplashRejectsInvalidScalarsAndAtlasBounds(t *testing.T) {
	tests := []struct {
		name  string
		entry string
		want  string
	}{
		{
			name:  "zero cell width",
			entry: `{"texture":"boot","cell_w":0,"cell_h":4,"columns":1,"frames":1}`,
			want:  "cell_w",
		},
		{
			name:  "zero cell height",
			entry: `{"texture":"boot","cell_w":4,"cell_h":0,"columns":1,"frames":1}`,
			want:  "cell_h",
		},
		{
			name:  "zero columns",
			entry: `{"texture":"boot","cell_w":4,"cell_h":4,"columns":0,"frames":1}`,
			want:  "columns 0",
		},
		{
			name:  "zero frames",
			entry: `{"texture":"boot","cell_w":4,"cell_h":4,"columns":1,"frames":0}`,
			want:  "frames 0",
		},
		{
			name:  "frames overflow uint16",
			entry: `{"texture":"boot","cell_w":1,"cell_h":1,"columns":1,"frames":65536}`,
			want:  "must be <= 65535",
		},
		{
			name:  "negative rate",
			entry: `{"texture":"boot","cell_w":4,"cell_h":4,"columns":1,"frames":1,"rate":-1}`,
			want:  "invalid rate",
		},
		{
			name:  "rate overflows float32",
			entry: `{"texture":"boot","cell_w":4,"cell_h":4,"columns":1,"frames":1,"rate":1e39}`,
			want:  "fit float32",
		},
		{
			name:  "negative min time",
			entry: `{"texture":"boot","cell_w":4,"cell_h":4,"columns":1,"frames":1,"min_time":-1}`,
			want:  "invalid min_time",
		},
		{
			name:  "negative fade",
			entry: `{"texture":"boot","cell_w":4,"cell_h":4,"columns":1,"frames":1,"fade":-1}`,
			want:  "invalid fade",
		},
		{
			name:  "anchor below viewport",
			entry: `{"texture":"boot","cell_w":4,"cell_h":4,"columns":1,"frames":1,"effect_anchor_x":-0.1}`,
			want:  "effect_anchor_x",
		},
		{
			name:  "anchor above viewport",
			entry: `{"texture":"boot","cell_w":4,"cell_h":4,"columns":1,"frames":1,"effect_anchor_y":1.1}`,
			want:  "effect_anchor_y",
		},
		{
			name:  "negative intensity",
			entry: `{"texture":"boot","cell_w":4,"cell_h":4,"columns":1,"frames":1,"effect_intensity":-1}`,
			want:  "effect_intensity",
		},
		{
			name:  "frames exceed texture width",
			entry: `{"texture":"boot","cell_w":4,"cell_h":4,"columns":4,"frames":4}`,
			want:  "requires 16x4 pixels",
		},
		{
			name:  "frames exceed texture height",
			entry: `{"texture":"boot","cell_w":4,"cell_h":4,"columns":3,"frames":7}`,
			want:  "requires 12x12 pixels",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := loadSplashEntry(t, 12, 8, tt.entry)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Load() error = %v, want substring %q", err, tt.want)
			}
		})
	}
}

func loadSplashEntry(t *testing.T, textureW, textureH int, entry string) (asset.Manifest, error) {
	t.Helper()
	manifestJSON := fmt.Sprintf(`{
		"version": 1,
		"textures": [{"name": "boot", "file": "boot.png"}],
		"splash": [%s]
	}`, entry)
	return asset.Load(fstest.MapFS{
		"manifest.json": {Data: []byte(manifestJSON)},
		"boot.png":      {Data: splashTestPNG(t, textureW, textureH)},
	})
}

func splashTestPNG(t *testing.T, width, height int) []byte {
	t.Helper()
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, image.NewNRGBA(image.Rect(0, 0, width, height))); err != nil {
		t.Fatalf("png.Encode() error = %v", err)
	}
	return encoded.Bytes()
}
