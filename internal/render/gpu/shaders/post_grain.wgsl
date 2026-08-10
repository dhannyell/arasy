// Mirrors platform/raster/postfx.go applyFilmGrainInto. Hash constants mirror
// particle.pcg3d and particle.Float01 in particle/hash.go, same as LensRain.
// The host folds the tick-quantized time into seed before upload, so the
// shader hashes a pure (x, y, tick) triple.
struct VertexOut {
  @builtin(position) position: vec4f,
  @location(0) uv: vec2f,
};

struct Params {
  values: vec4f,
  // state packs the target dimensions; seed shares its 16-byte slot so the
  // host writes the raw u32 at offset 28 without a float round-trip.
  state: vec3f,
  seed: u32,
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

fn pcg3d(p: vec3u) -> vec3u {
  var v = p * 1664525u + 1013904223u;
  v.x += v.y * v.z;
  v.y += v.z * v.x;
  v.z += v.x * v.y;
  v ^= v >> vec3u(16u);
  v.x += v.y * v.z;
  v.y += v.z * v.x;
  v.z += v.x * v.y;
  return v;
}

fn float01(value: u32) -> f32 {
  return f32(value) * 2.3283064365386963e-10;
}

@fragment
fn fs_main(in: VertexOut) -> @location(0) vec4f {
  let source = textureSampleLevel(srcTexture, srcSampler, in.uv, 0.0);
  let intensity = params.values.x;
  if (intensity == 0.0) {
    return source;
  }
  let grainSize = select(1.0, params.values.y, params.values.y > 0.0);
  let dims = params.state.yz;
  let pixel = floor(floor(in.uv * dims) / grainSize);
  let hash = pcg3d(vec3u(u32(pixel.x), u32(pixel.y), params.seed));
  // Two hash lanes give a triangular distribution: most grains are weak, a
  // few are strong, like photographic grain.
  let noise = float01(hash.x) + float01(hash.y) - 1.0;
  // Grain peaks in the mid-tones and fades in deep shadow and highlight,
  // where film shows almost none.
  let luma = dot(source.rgb, vec3f(0.2126, 0.7152, 0.0722));
  let grain = noise * intensity * clamp(luma * (1.0 - luma) * 4.0, 0.0, 1.0);
  return vec4f(source.rgb + vec3f(grain), source.a);
}
