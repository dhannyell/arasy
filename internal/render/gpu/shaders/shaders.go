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
