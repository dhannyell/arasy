package gpu

import "math"

// TextureResources are the handles produced while uploading a declared
// texture. Mipmap generation remains in the js-side command encoder.
type TextureResources struct {
	Texture   Handle
	View      Handle
	BindGroup Handle
}

// BuildTextureResources performs the testable part of declared-texture setup.
func BuildTextureResources(dev Device, pix []byte, w, h int, pipeline, sampler Handle) TextureResources {
	levels := MipLevelCount(w, h)
	texture := dev.CreateTexture(TextureDesc(w, h, levels))
	dev.WriteTexture(
		map[string]any{"texture": texture}, pix,
		TextureWriteLayout(4*w, h), TextureWriteSize(w, h),
	)
	view := dev.CreateView(texture, nil)
	layout := dev.BindGroupLayout(pipeline, 0)
	bindGroup := dev.CreateBindGroup(TextureBindGroupDesc(layout, view, sampler))
	return TextureResources{Texture: texture, View: view, BindGroup: bindGroup}
}

// MipLevelCount returns floor(log2(max(w, h))) + 1.
func MipLevelCount(w, h int) int {
	max := w
	if h > max {
		max = h
	}
	if max <= 1 {
		return 1
	}
	return int(math.Floor(math.Log2(float64(max)))) + 1
}
