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

// textWGSL is the atlas-backed text WebGPU pipeline shader.
//
// Vertex inputs match tess.TextVertex serialized by tess.TextBytes:
// @location(0) position vec2f, @location(1) color vec4f,
// @location(2) uv vec2f.
//
//go:embed shaders/text.wgsl
var textWGSL string
