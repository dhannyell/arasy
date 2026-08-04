package gpu

// AlphaBlend is straight-alpha blending for solid colors.
func AlphaBlend() map[string]any {
	return map[string]any{
		"color": map[string]any{
			"srcFactor": "src-alpha",
			"dstFactor": "one-minus-src-alpha",
			"operation": "add",
		},
		"alpha": map[string]any{
			"srcFactor": "one",
			"dstFactor": "one-minus-src-alpha",
			"operation": "add",
		},
	}
}

// PremultipliedBlend is source-over blending for premultiplied pixels.
func PremultipliedBlend() map[string]any {
	return map[string]any{
		"color": map[string]any{
			"srcFactor": "one",
			"dstFactor": "one-minus-src-alpha",
			"operation": "add",
		},
		"alpha": map[string]any{
			"srcFactor": "one",
			"dstFactor": "one-minus-src-alpha",
			"operation": "add",
		},
	}
}

// AdditiveStraightBlend adds premultiplied source color to the destination.
// is the additive operator for pipelines whose shaders
// output straight (non-premultiplied) alpha: the source color is scaled by
// its own alpha before being summed, so a faint glow adds faintly.
func AdditiveStraightBlend() map[string]any {
	return map[string]any{
		"color": map[string]any{
			"srcFactor": "src-alpha",
			"dstFactor": "one",
			"operation": "add",
		},
		"alpha": map[string]any{
			"srcFactor": "one",
			"dstFactor": "one",
			"operation": "add",
		},
	}
}

func AdditiveBlend() map[string]any {
	return map[string]any{
		"color": map[string]any{
			"srcFactor": "one",
			"dstFactor": "one",
			"operation": "add",
		},
		"alpha": map[string]any{
			"srcFactor": "one",
			"dstFactor": "one",
			"operation": "add",
		},
	}
}

func SolidPipelineDesc(module Handle, format string, sampleCount int, blend map[string]any) map[string]any {
	return pipelineDesc(module, format, sampleCount, 24, []any{
		map[string]any{"shaderLocation": 0, "offset": 0, "format": "float32x2"},
		map[string]any{"shaderLocation": 1, "offset": 8, "format": "float32x4"},
	}, blend)
}

func TexturedPipelineDesc(module Handle, format string, sampleCount int, blend map[string]any) map[string]any {
	return pipelineDesc(module, format, sampleCount, 40, []any{
		map[string]any{"shaderLocation": 0, "offset": 0, "format": "float32x4"},
		map[string]any{"shaderLocation": 1, "offset": 16, "format": "float32x4"},
		map[string]any{"shaderLocation": 2, "offset": 32, "format": "float32x2"},
	}, blend)
}

// SpritePipelineDesc expands storage-buffer sprite records with vertex
// pulling, so it intentionally has no vertex buffer layout.
func SpritePipelineDesc(module Handle, format string, sampleCount int, blend map[string]any) map[string]any {
	return map[string]any{
		"layout":      "auto",
		"vertex":      map[string]any{"module": module, "entryPoint": "vs_main"},
		"fragment":    map[string]any{"module": module, "entryPoint": "fs_main", "targets": []any{map[string]any{"format": format, "blend": blend}}},
		"primitive":   map[string]any{"topology": "triangle-list"},
		"multisample": map[string]any{"count": sampleCount},
	}
}

// TextGlyphPipelineDesc expands storage-buffer glyph records without a
// vertex buffer. Unlike sprites, glyph fragments carry straight RGB plus an
// alpha mask, so they must retain the text pipeline's straight-alpha blend.
func TextGlyphPipelineDesc(module Handle, format string, sampleCount int, blend map[string]any) map[string]any {
	return map[string]any{
		"layout":      "auto",
		"vertex":      map[string]any{"module": module, "entryPoint": "vs_main"},
		"fragment":    map[string]any{"module": module, "entryPoint": "fs_main", "targets": []any{map[string]any{"format": format, "blend": blend}}},
		"primitive":   map[string]any{"topology": "triangle-list"},
		"multisample": map[string]any{"count": sampleCount},
	}
}

func pipelineDesc(module Handle, format string, sampleCount, stride int, attributes []any, blend map[string]any) map[string]any {
	return map[string]any{
		"layout": "auto",
		"vertex": map[string]any{
			"module": module, "entryPoint": "vs_main",
			"buffers": []any{map[string]any{"arrayStride": stride, "attributes": attributes}},
		},
		"fragment": map[string]any{
			"module": module, "entryPoint": "fs_main",
			"targets": []any{map[string]any{"format": format, "blend": blend}},
		},
		"primitive":   map[string]any{"topology": "triangle-list"},
		"multisample": map[string]any{"count": sampleCount},
	}
}

// MipPipelineDesc describes the boot-time fullscreen mipmap pipeline.
func MipPipelineDesc(module Handle) map[string]any {
	return map[string]any{
		"layout": "auto",
		"vertex": map[string]any{"module": module, "entryPoint": "vs_main"},
		"fragment": map[string]any{
			"module": module, "entryPoint": "fs_main",
			"targets": []any{map[string]any{"format": "rgba8unorm"}},
		},
		"primitive": map[string]any{"topology": "triangle-list"},
	}
}

func TextureSamplerDesc() map[string]any {
	return map[string]any{"magFilter": "linear", "minFilter": "linear", "mipmapFilter": "linear", "maxAnisotropy": 16}
}

func MipSamplerDesc() map[string]any {
	return map[string]any{"magFilter": "linear", "minFilter": "linear"}
}

func NearestSamplerDesc() map[string]any {
	return map[string]any{"magFilter": "nearest", "minFilter": "nearest"}
}

// LinearSamplerDesc is used for rasterized TTF/OpenType text atlases. The
// atlas has transparent padding around each glyph, so linear sampling does
// not bleed neighboring glyphs into the result.
func LinearSamplerDesc() map[string]any {
	return map[string]any{"magFilter": "linear", "minFilter": "linear"}
}

func TextureDesc(w, h, levels int) map[string]any {
	return map[string]any{
		"size": []any{w, h}, "format": "rgba8unorm", "mipLevelCount": levels,
		"usage": TextureUsageTextureBinding | TextureUsageCopyDst | TextureUsageRenderAttachment,
	}
}

// TextureArrayDesc describes a uniform stack of 2D texture layers. Sprite
// records select their layer in the shader, avoiding a bind-group switch for
// every interleaved texture.
func TextureArrayDesc(w, h, layers, levels int) map[string]any {
	return map[string]any{
		"size":   map[string]any{"width": w, "height": h, "depthOrArrayLayers": layers},
		"format": "rgba8unorm", "mipLevelCount": levels,
		"usage": TextureUsageTextureBinding | TextureUsageCopyDst | TextureUsageRenderAttachment,
	}
}

func R8TextureDesc(w, h int) map[string]any {
	return map[string]any{
		"size": []any{w, h}, "format": "r8unorm",
		"usage": TextureUsageTextureBinding | TextureUsageCopyDst,
	}
}

func MSAATextureDesc(w, h int, format string, sampleCount int) map[string]any {
	return map[string]any{
		"size": []any{w, h}, "sampleCount": sampleCount, "format": format,
		"usage": TextureUsageRenderAttachment,
	}
}

func VertexBufferDesc(size int) map[string]any {
	return BufferDesc(size, BufferUsageVertex|BufferUsageCopyDst)
}

func BufferDesc(size, usage int) map[string]any {
	return map[string]any{"size": size, "usage": usage}
}

func TextureWriteLayout(bytesPerRow, rowsPerImage int) map[string]any {
	return map[string]any{"bytesPerRow": bytesPerRow, "rowsPerImage": rowsPerImage}
}

func TextureWriteSize(w, h int) map[string]any {
	return map[string]any{"width": w, "height": h, "depthOrArrayLayers": 1}
}

func TextureBindGroupDesc(layout, view, sampler Handle) map[string]any {
	return map[string]any{
		"layout": layout,
		"entries": []any{
			map[string]any{"binding": 0, "resource": view},
			map[string]any{"binding": 1, "resource": sampler},
		},
	}
}
