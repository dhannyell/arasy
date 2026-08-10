// Mirrors platform/raster/postfx.go applyLensRain. Hash constants mirror
// particle.pcg3d and particle.Float01 in particle/hash.go.
struct VertexOut {
  @builtin(position) position: vec4f,
  @location(0) uv: vec2f,
};

struct Params {
  values: vec4f,
  // state packs time and the target dimensions; seed shares its 16-byte slot
  // so the host writes the raw u32 at offset 28 without a float round-trip.
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
  let density = clamp(params.values.x, 0.0, 1.0);
  if (density == 0.0) {
    return source;
  }
  let size = select(48.0, params.values.y, params.values.y > 0.0);
  let pixelPosition = in.uv * params.state.yz;
  var color = source.rgb;
  color = lensRainLayer(in.uv, pixelPosition, source, color, density * 0.70, size * 0.70,
    params.values.z * 1.20, params.values.w * 0.70, params.seed + 0x13u,
    params.state.x, 0.18, 0.28, 0.85);
  color = lensRainLayer(in.uv, pixelPosition, source, color, density * 0.45, size * 1.35,
    params.values.z * 0.80, params.values.w, params.seed + 0x29u,
    params.state.x, 0.22, 0.34, 0.90);
  color = lensRainLayer(in.uv, pixelPosition, source, color, density * 0.18, size * 3.00,
    params.values.z * 0.55, params.values.w * 1.25, params.seed + 0x41u,
    params.state.x, 0.13, 0.42, 0.65);
  return vec4f(color, source.a);
}

fn lensRainLayer(uv: vec2f, pixelPosition: vec2f, source: vec4f, pixel: vec3f,
  density: f32, size: f32, speed: f32, refraction: f32, seed: u32, time: f32,
  radiusXScale: f32, radiusYScale: f32, opacity: f32) -> vec3f {
  let cell = floor(pixelPosition / size);
  let hash = pcg3d(vec3u(u32(cell.x), u32(cell.y), seed));
  let h = vec3f(float01(hash.x), float01(hash.y), float01(hash.z));
  if (h.x >= density) {
    return pixel;
  }

  let center = (cell + vec2f(h.y, fract(h.x - time * speed * (0.6 + 0.8 * h.x)))) * size;
  let radiusX = size * (radiusXScale + 0.10 * h.z);
  let radiusY = size * (radiusYScale + 0.18 * h.z);
  let delta = pixelPosition + vec2f(0.5) - center;
  let distance = length(delta);
  let shapeX = delta.x / radiusX - (h.y - 0.5) * 0.20 * (delta.y / radiusY);
  let shapeY = delta.y / radiusY;
  let normalizedDistance = length(vec2f(shapeX, shapeY));
  let mask = smoothstep(1.0, 0.80, normalizedDistance);
  if (mask == 0.0) {
    return pixel;
  }

  var result = source.rgb;
  if (refraction > 0.0 && distance > 0.0) {
    let offset = delta * ((1.0 - normalizedDistance) * refraction * size * 0.65 / distance);
    result = textureSampleLevel(srcTexture, srcSampler, uv - offset / params.state.yz, 0.0).rgb;
  }

  let highlightDistance = length(vec2f(shapeX + 0.28, shapeY + 0.34));
  let highlight = smoothstep(0.54, 0.0, highlightDistance) * mask;
  let rim = smoothstep(0.48, 0.92, normalizedDistance) *
    (1.0 - smoothstep(0.92, 1.0, normalizedDistance));
  let shadow = 0.10 * mask + 0.48 * rim;
  result *= 1.0 - shadow;
  result += vec3f(0.16, 0.19, 0.24) * highlight;
  return pixel + (result - pixel) * (mask * opacity);
}
