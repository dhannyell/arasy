//go:build js && wasm

package wasm

import _ "embed"

// solidWGSL is the solid-color WebGPU pipeline shader.
//
// Vertex inputs match tess.Vertex serialized by tess.Bytes:
// @location(0) position vec2f, @location(1) color vec4f.
//
//go:embed shaders/solid.wgsl
var solidWGSL string

// textGlyphWGSL expands storage-buffer glyph records while retaining all text
// layout decisions in the tessellator.
//
//go:embed shaders/text_instanced.wgsl
var textGlyphWGSL string

// texturedWGSL is the image/texture WebGPU pipeline shader.
//
// Vertex inputs match tess.TexVertex serialized by tess.TexBytes:
// @location(0) position vec4f (xw, yw, 0, w — pre-multiplied by w so the
// hardware perspective divide and UV interpolation are perspective-correct),
// @location(1) tint vec4f, @location(2) uv vec2f.
//
//go:embed shaders/textured.wgsl
var texturedWGSL string

// spriteWGSL expands storage-buffer sprite records in the vertex shader.
//
//go:embed shaders/sprite_render.wgsl
var spriteWGSL string

// spriteArrayWGSL is the array-texture variant of the sprite pipeline. Each
// record selects a layer, so interleaved textures retain painter order in one
// draw call.
//
//go:embed shaders/sprite_array_render.wgsl
var spriteArrayWGSL string

// mipmapWGSL is the internal fullscreen-triangle shader used to generate a
// texture's mip chain via successive render passes. It has no vertex buffer
// inputs; the fullscreen triangle is derived from @builtin(vertex_index).
//
//go:embed shaders/mipmap.wgsl
var mipmapWGSL string

//go:embed shaders/particle_compute.wgsl
var particlesComputeWGSL string

//go:embed shaders/particle_render.wgsl
var particlesRenderWGSL string

// particlesCompositeWGSL blends one particle offscreen target into the main
// pass as a fullscreen triangle. Used only when Config.ParticleOffscreenScale
// enables offscreen particle rendering.
//
//go:embed shaders/particle_composite.wgsl
var particlesCompositeWGSL string
