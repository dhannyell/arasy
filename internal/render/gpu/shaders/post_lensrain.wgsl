// Mirrors platform/raster/postfx.go applyLensRain. Hash constants mirror
// particle.pcg3d and particle.Float01 in particle/hash.go.
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
  let density = clamp(params.values.x, 0.0, 1.0);
  if (density == 0.0) {
    return source;
  }
  let size = select(48.0, params.values.y, params.values.y > 0.0);
  let pixel = in.uv * params.state.yz;
  let cell = floor(pixel / size);
  let cellBits = vec3u(u32(cell.x), u32(cell.y), u32(params.state.w));
  let hash = pcg3d(cellBits);
  let h = vec3f(float01(hash.x), float01(hash.y), float01(hash.z));
  if (h.x >= density) {
    return source;
  }
  let center = (cell + vec2f(h.y, fract(h.x - params.state.x * params.values.z * (0.6 + 0.8 * h.x)))) * size;
  let radius = size * (0.12 + 0.08 * h.z);
  let delta = pixel - center;
  let distance = length(delta);
  let mask = clamp(1.0 - distance / max(radius, 1.0), 0.0, 1.0);
  if (mask == 0.0) {
    return source;
  }
  let safeDistance = max(distance, 1.0);
  let offset = delta * ((1.0 - distance / max(radius, 1.0)) * params.values.w * size / safeDistance);
  let refracted = textureSampleLevel(srcTexture, srcSampler, in.uv - offset / params.state.yz, 0.0);
  let result = mix(source, refracted, select(0.0, 1.0, params.values.w > 0.0 && distance > 0.0));
  return vec4f(mix(result.rgb, result.rgb * 0.55 + vec3f(0.32), mask), result.a);
}
