// Mirrors platform/raster/postfx.go applyRadialBlurInto: each pixel averages
// eight bilinear taps streaked toward the focus point.
struct VertexOut {
  @builtin(position) position: vec4f,
  @location(0) uv: vec2f,
};

struct Params {
  // values packs center.xy in screen pixels, strength.
  values: vec4f,
  // state packs the target dimensions in yz.
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
  let strength = clamp(params.values.z, 0.0, 1.0);
  if (strength == 0.0) {
    return source;
  }
  let dims = params.state.yz;
  let centerUV = params.values.xy / dims;
  let duv = (centerUV - in.uv) * strength * 0.15;
  var acc = vec4f(0.0);
  for (var i = 0; i < 8; i++) {
    let t = f32(i) / 7.0;
    acc += textureSampleLevel(srcTexture, srcSampler, in.uv + duv * t, 0.0);
  }
  return acc / 8.0;
}
