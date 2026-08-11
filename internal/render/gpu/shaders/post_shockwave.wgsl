// Mirrors platform/raster/postfx.go applyShockwave.
struct VertexOut {
  @builtin(position) position: vec4f,
  @location(0) uv: vec2f,
};

struct Params {
  values: vec4f,
  state: vec4f,
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
  let dimensions = params.state.yz;
  let pixel = in.uv * dimensions;
  let direction = pixel - params.values.xy;
  let distance = length(direction);
  let radius = params.values.z;
  let width = params.values.w;
  let source = textureSampleLevel(srcTexture, srcSampler, in.uv, 0.0);
  if (width <= 0.0 || distance <= 0.0) {
    return source;
  }
  let band = max(0.0, 1.0 - abs(distance - radius) / width);
  if (band <= 0.0) {
    return source;
  }
  let offset = direction * (band * band * params.state.x / distance);
  return textureSampleLevel(srcTexture, srcSampler, in.uv - offset / dimensions, 0.0);
}
