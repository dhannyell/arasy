// Mirrors platform/raster/postfx.go applyCircleWipeInto: the visible circle
// shrinks from the farthest screen corner to zero as progress goes 0..1.
struct VertexOut {
  @builtin(position) position: vec4f,
  @location(0) uv: vec2f,
};

struct Params {
  color: vec4f,
  // values packs center.xy, progress, softness.
  values: vec4f,
  // state packs the target dimensions.
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
  let source = textureSampleLevel(srcTexture, srcSampler, in.uv, 0.0);
  let dims = params.state.xy;
  let center = params.values.xy;
  let progress = clamp(params.values.z, 0.0, 1.0);
  let softness = clamp(params.values.w, 0.0, 1.0);

  var farthest = 0.0;
  farthest = max(farthest, distance(vec2f(0.0, 0.0), center));
  farthest = max(farthest, distance(vec2f(dims.x, 0.0), center));
  farthest = max(farthest, distance(vec2f(0.0, dims.y), center));
  farthest = max(farthest, distance(vec2f(dims.x, dims.y), center));

  let radius = farthest * (1.0 - progress);
  let edge0 = radius * (1.0 - softness);
  let pixel = in.uv * dims;
  let t = smoothstep(edge0, radius, distance(pixel, center));
  return vec4f(mix(source.rgb, params.color.rgb, t), source.a);
}
