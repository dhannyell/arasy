// Mirrors platform/raster/postfx.go applyScanlinesInto: darkens periodic
// horizontal lines with a sin^2 profile for a CRT monitor look.
struct VertexOut {
  @builtin(position) position: vec4f,
  @location(0) uv: vec2f,
};

struct Params {
  // values packs intensity, thickness, maskStrength.
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
  let intensity = clamp(params.values.x, 0.0, 1.0);
  let mask = clamp(params.values.z, 0.0, 1.0);
  if (intensity == 0.0 && mask == 0.0) {
    return source;
  }
  let period = select(3.0, params.values.y, params.values.y > 0.0);
  let dims = params.state.yz;
  let py = floor(in.uv.y * dims.y) + 0.5;
  let s = sin(3.141592653589793 * py / period);
  let factor = 1.0 - intensity * s * s;
  // Aperture grille: each pixel column favors one triad channel and dims
  // the other two by mask.
  let dim = 1.0 - mask;
  let phase = u32(floor(in.uv.x * dims.x)) % 3u;
  var gains = vec3f(dim);
  gains[phase] = 1.0;
  return vec4f(source.rgb * factor * gains, source.a);
}
