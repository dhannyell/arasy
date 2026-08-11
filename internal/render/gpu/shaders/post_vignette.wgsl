// Mirrors platform/raster/postfx.go applyVignette.
struct VertexOut {
  @builtin(position) position: vec4f,
  @location(0) uv: vec2f,
};

struct Params {
  color: vec4f,
  values: vec4f,
};

@group(0) @binding(0) var srcTexture: texture_2d<f32>;
@group(0) @binding(1) var srcSampler: sampler;
@group(0) @binding(2) var<uniform> params: Params;

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
  let source = textureSampleLevel(srcTexture, srcSampler, in.uv, 0.0);
  let p = in.uv * 2.0 - 1.0;
  let distance = length(p) / sqrt(2.0);
  let strength = clamp(params.values.x, 0.0, 1.0);
  let softness = clamp(params.values.y, 0.0, 1.0);
  let t = strength * smoothstep(1.0 - softness, 1.0, distance);
  return vec4f(mix(source.rgb, params.color.rgb, t), source.a);
}
