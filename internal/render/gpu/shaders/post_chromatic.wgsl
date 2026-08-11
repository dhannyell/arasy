// Mirrors platform/raster/postfx.go applyChromaticAberration.
struct VertexOut {
  @builtin(position) position: vec4f,
  @location(0) uv: vec2f,
};

struct Params {
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
  let strength = params.values.x;
  let dimensions = params.values.yz;
  let p = in.uv * 2.0 - 1.0;
  let offset = p * dot(p, p) * strength;
  let red = textureSampleLevel(srcTexture, srcSampler, in.uv + offset / dimensions, 0.0).r;
  let blue = textureSampleLevel(srcTexture, srcSampler, in.uv - offset / dimensions, 0.0).b;
  return vec4f(red, source.g, blue, source.a);
}
