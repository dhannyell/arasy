package wasm

import (
	"testing"

	"github.com/dhannyell/arasy/internal/render/gpu/tess"
)

func TestPlanSpriteTextureArray(t *testing.T) {
	tests := []struct {
		name      string
		sizes     tess.TexSizes
		maxLayers int
		want      spriteTextureArrayPlan
	}{
		{
			name:      "empty manifest",
			sizes:     tess.TexSizes{{}},
			maxLayers: 256,
			want:      spriteTextureArrayPlan{Reason: "fewer than two declared textures"},
		},
		{
			name:      "one texture",
			sizes:     tess.TexSizes{{}, {64, 64}},
			maxLayers: 256,
			want:      spriteTextureArrayPlan{Reason: "fewer than two declared textures"},
		},
		{
			name:      "matching textures",
			sizes:     tess.TexSizes{{}, {64, 64}, {64, 64}, {64, 64}},
			maxLayers: 256,
			want:      spriteTextureArrayPlan{Enabled: true, Width: 64, Height: 64, Layers: 3},
		},
		{
			name:      "failed decode keeps stable layer indexes",
			sizes:     tess.TexSizes{{}, {64, 64}, {}, {64, 64}},
			maxLayers: 256,
			want:      spriteTextureArrayPlan{Enabled: true, Width: 64, Height: 64, Layers: 3},
		},
		{
			name:      "different dimensions",
			sizes:     tess.TexSizes{{}, {64, 64}, {128, 64}},
			maxLayers: 256,
			want:      spriteTextureArrayPlan{Reason: "texture dimensions differ"},
		},
		{
			name:      "device layer limit",
			sizes:     tess.TexSizes{{}, {64, 64}, {64, 64}, {64, 64}},
			maxLayers: 2,
			want:      spriteTextureArrayPlan{Reason: "texture count exceeds device layer limit"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := planSpriteTextureArray(tt.sizes, tt.maxLayers); got != tt.want {
				t.Fatalf("planSpriteTextureArray() = %+v, want %+v", got, tt.want)
			}
		})
	}
}
