// Package shaders holds the shared WGSL sources consumed by the WASM WebGPU
// host and, in Phase C, by the native host.
package shaders

import _ "embed"

// SolidWGSL is the solid-color WebGPU pipeline shader.
//
// Vertex inputs match tess.Vertex serialized by tess.Bytes:
// @location(0) position vec2f, @location(1) color vec4f.
//
//go:embed solid.wgsl
var SolidWGSL string

// TextGlyphWGSL expands storage-buffer glyph records while retaining all text
// layout decisions in the tessellator.
//
//go:embed text_instanced.wgsl
var TextGlyphWGSL string

// TexturedWGSL is the image/texture WebGPU pipeline shader.
//
// Vertex inputs match tess.TexVertex serialized by tess.TexBytes:
// @location(0) position vec4f (xw, yw, 0, w — pre-multiplied by w so the
// hardware perspective divide and UV interpolation are perspective-correct),
// @location(1) tint vec4f, @location(2) uv vec2f.
//
//go:embed textured.wgsl
var TexturedWGSL string

// SpriteWGSL expands storage-buffer sprite records in the vertex shader.
//
//go:embed sprite_render.wgsl
var SpriteWGSL string

// SpriteArrayWGSL is the array-texture variant of the sprite pipeline. Each
// record selects a layer, so interleaved textures retain painter order in one
// draw call.
//
//go:embed sprite_array_render.wgsl
var SpriteArrayWGSL string

// SpriteBatchAnimWGSL draws retained animated sprite batches (single
// texture): flip-book and bounce/pulse run in the vertex shader from the
// retained base records, per-instance animation records, and one dynamic-
// offset uniform slot per draw command.
//
//go:embed sprite_batch_anim.wgsl
var SpriteBatchAnimWGSL string

// SpriteBatchAnimArrayWGSL is the texture-array variant of the retained
// animated sprite batch pipeline.
//
//go:embed sprite_batch_array_anim.wgsl
var SpriteBatchAnimArrayWGSL string

// MipmapWGSL is the internal fullscreen-triangle shader used to generate a
// texture's mip chain via successive render passes. It has no vertex buffer
// inputs; the fullscreen triangle is derived from @builtin(vertex_index).
//
//go:embed mipmap.wgsl
var MipmapWGSL string

//go:embed particle_compute.wgsl
var ParticlesComputeWGSL string

//go:embed particle_render.wgsl
var ParticlesRenderWGSL string

// ParticlesCompositeWGSL blends one particle offscreen target into the main
// pass as a fullscreen triangle. Used only when Config.ParticleOffscreenScale
// enables offscreen particle rendering.
//
//go:embed particle_composite.wgsl
var ParticlesCompositeWGSL string

// PostBlitWGSL copies a post-process target into another render attachment.
//
//go:embed post_blit.wgsl
var PostBlitWGSL string

// PostVignetteWGSL mirrors platform/raster/postfx.go applyVignette.
//
//go:embed post_vignette.wgsl
var PostVignetteWGSL string

// PostColorGradeWGSL mirrors platform/raster/postfx.go applyColorGrade.
//
//go:embed post_colorgrade.wgsl
var PostColorGradeWGSL string

// PostPixelateWGSL mirrors platform/raster/postfx.go applyPixelate.
//
//go:embed post_pixelate.wgsl
var PostPixelateWGSL string

// PostChromaticWGSL mirrors platform/raster/postfx.go applyChromaticAberration.
//
//go:embed post_chromatic.wgsl
var PostChromaticWGSL string

// PostBloomWGSL mirrors platform/raster/postfx.go bloom bright, downsample,
// and upsample passes.
//
//go:embed post_bloom.wgsl
var PostBloomWGSL string

// PostBloomCombineWGSL mirrors the final bloom combine pass.
//
//go:embed post_bloom_combine.wgsl
var PostBloomCombineWGSL string

// PostShockwaveWGSL mirrors platform/raster/postfx.go applyShockwave.
//
//go:embed post_shockwave.wgsl
var PostShockwaveWGSL string

// PostLensRainWGSL mirrors platform/raster/postfx.go applyLensRain.
//
//go:embed post_lensrain.wgsl
var PostLensRainWGSL string
