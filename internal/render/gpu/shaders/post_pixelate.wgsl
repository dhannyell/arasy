// Mirrors platform/raster/postfx.go applyPixelate.
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
  let size = params.values.x;
  if (size <= 0.0) {
    return textureSampleLevel(srcTexture, srcSampler, in.uv, 0.0);
  }
  let dimensions = params.values.yz;
  let pixel = in.uv * dimensions;
  let source = floor(pixel / size) * size + vec2f(size * 0.5);
  let clamped = clamp(source, vec2f(0.0), dimensions - vec2f(1.0));
  return textureSampleLevel(srcTexture, srcSampler, (clamped + vec2f(0.5)) / dimensions, 0.0);
}
