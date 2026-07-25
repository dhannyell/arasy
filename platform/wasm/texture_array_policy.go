package wasm

import "github.com/dhannyell/arasy/internal/render/gpu/tess"

type spriteTextureArrayPlan struct {
	Enabled bool
	Width   int
	Height  int
	Layers  int
	Reason  string
}

// planSpriteTextureArray selects the shared-array resource layout only when
// at least two decoded textures have identical dimensions and the complete
// TextureID range fits the device's layer limit. Zero-size entries represent
// failed decodes and do not prevent valid textures from sharing the array.
func planSpriteTextureArray(sizes tess.TexSizes, maxLayers int) spriteTextureArrayPlan {
	layers := len(sizes) - 1
	if layers <= 1 {
		return spriteTextureArrayPlan{Reason: "fewer than two declared textures"}
	}
	if maxLayers <= 0 || layers > maxLayers {
		return spriteTextureArrayPlan{Reason: "texture count exceeds device layer limit"}
	}

	width, height, available := 0, 0, 0
	for _, size := range sizes[1:] {
		if size[0] <= 0 || size[1] <= 0 {
			continue
		}
		available++
		if width == 0 {
			width, height = size[0], size[1]
			continue
		}
		if size[0] != width || size[1] != height {
			return spriteTextureArrayPlan{Reason: "texture dimensions differ"}
		}
	}
	if available < 2 {
		return spriteTextureArrayPlan{Reason: "fewer than two decoded textures"}
	}

	return spriteTextureArrayPlan{
		Enabled: true,
		Width:   width,
		Height:  height,
		Layers:  layers,
	}
}
