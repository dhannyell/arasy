// Mirrors platform/raster/postfx.go applyFocusBlurInto. The dual-Kawase
// pyramid reuses post_bloom.wgsl's fs_down/fs_up entry points; this file
// only holds the final mix, weighted by distance to the nearest focus point.
struct VertexOut {
  @builtin(position) position: vec4f,
  @location(0) uv: vec2f,
};

// values: strength, falloff px, point count, corner rounding px.
// state: x and w are the sharp box's half-size, yz the frame size.
// points: screen positions, two per vec4f, at the shared table offset.
struct Params {
  values: vec4f,
  state: vec4f,
  pad: array<vec4f, 2>,
  points: array<vec4f, 8>,
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

// roundedBoxDistance mirrors platform/raster/postfx.go roundedBoxDistance.
fn roundedBoxDistance(offset: vec2f, half: vec2f, rounding: f32) -> f32 {
  let q = abs(offset) - (half - vec2f(rounding));
  return length(max(q, vec2f(0.0))) + min(max(q.x, q.y), 0.0) - rounding;
}

@fragment
fn fs_main(in: VertexOut) -> @location(0) vec4f {
  let source = textureSampleLevel(sourceTexture, srcSampler, in.uv, 0.0);
  let blurred = textureSampleLevel(blurTexture, srcSampler, in.uv, 0.0);
  let strength = clamp(params.values.x, 0.0, 1.0);
  let falloff = max(params.values.y, 1e-3);
  let count = u32(params.values.z);
  let rounding = params.values.w;
  let half = params.state.xw;

  var weight = strength;
  if (count > 0u) {
    let px = in.uv * params.state.yz;
    var nearest = 1e30;
    for (var i = 0u; i < count; i = i + 1u) {
      let p = params.points[i >> 1u];
      let point = select(p.xy, p.zw, (i & 1u) == 1u);
      nearest = min(nearest, roundedBoxDistance(px - point, half, rounding));
    }
    weight = strength * smoothstep(0.0, falloff, nearest);
  }
  return vec4f(mix(source.rgb, blurred.rgb, weight), mix(source.a, blurred.a, weight));
}
