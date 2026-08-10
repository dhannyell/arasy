// Mirrors platform/raster/postfx.go applyBlurInto. The dual-Kawase pyramid
// itself reuses post_bloom.wgsl's fs_down/fs_up entry points (identical taps
// and weights); this file only holds the final sharp/blurred mix.
struct VertexOut {
  @builtin(position) position: vec4f,
  @location(0) uv: vec2f,
};

struct Params {
  values: vec4f,
};

@group(0) @binding(0) var sourceTexture: texture_2d<f32>;
@group(0) @binding(1) var blurTexture: texture_2d<f32>;
@group(0) @binding(2) var srcSampler: sampler;
@group(0) @binding(3) var<uniform> params: Params;

@vertex
fn vs_main(@builtin(vertex_index) vertexIndex: u32) -> VertexOut {
  var out: VertexOut;
  let x = f32((vertexIndex << 1u) & 2u);
  let y = f32(vertexIndex & 2u);
  out.uv = vec2f(x, y);
  out.position = vec4f(x * 2.0 - 1.0, 1.0 - y * 2.0, 0.0, 1.0);
  return out;
}

@fragment
fn fs_main(in: VertexOut) -> @location(0) vec4f {
  let source = textureSampleLevel(sourceTexture, srcSampler, in.uv, 0.0);
  let blurred = textureSampleLevel(blurTexture, srcSampler, in.uv, 0.0);
  let strength = clamp(params.values.x, 0.0, 1.0);
  return vec4f(mix(source.rgb, blurred.rgb, strength), mix(source.a, blurred.a, strength));
}
