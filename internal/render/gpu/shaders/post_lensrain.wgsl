// Defines the LensRain behavior; platform/raster/postfx.go applyLensRainInto
// mirrors this file as the budgeted reference (layered oracle). Hash
// constants mirror particle.pcg3d and particle.Float01 in particle/hash.go.
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

fn rainHash(cellX: f32, cellY: f32, seed: u32) -> vec3f {
  let h = pcg3d(vec3u(u32(cellX), u32(cellY), seed));
  return vec3f(float01(h.x), float01(h.y), float01(h.z));
}

// rainDrop shades one dome-shaped droplet: the background bends inward
// through the dome (strong at the rim), with a thin dark edge and a small
// specular highlight. Zero refraction keeps the pixel sample local (N1).
// lightScale flattens the dome lighting as an eroding droplet dries out.
fn rainDrop(uv: vec2f, pp: vec2f, pixel: vec3f, center: vec2f, radii: vec2f,
  refraction: f32, opacity: f32, lightScale: f32, shape: f32, dims: vec2f) -> vec3f {
  let delta = pp + vec2f(0.5) - center;
  var n = delta / radii;
  // Gravity asymmetry: the drop bulges below its center and thins above it.
  n.y = select(n.y * 1.12, n.y * 0.82, n.y > 0.0);
  // Angular wobble breaks the perfect circle; shape picks the per-drop phase.
  let angle = atan2(n.y, n.x);
  n *= 1.0 + 0.09 * sin(3.0 * angle + shape * 6.2832) + 0.05 * sin(5.0 * angle + shape * 12.9);
  let d2 = dot(n, n);
  if (d2 >= 1.0) {
    return pixel;
  }

  let d = sqrt(d2);
  let dome = sqrt(1.0 - d2);
  let mask = smoothstep(1.0, 0.85, d) * opacity;
  if (mask <= 0.0) {
    return pixel;
  }

  var result = pixel;
  if (refraction > 0.0) {
    let bend = refraction * (0.45 + 0.55 * d / max(dome, 0.25));
    result = textureSampleLevel(srcTexture, srcSampler, uv - delta * bend / dims, 0.0).rgb;
  }

  let shade = 1.0 - 0.26 * smoothstep(0.7, 1.0, d);
  let highlight = smoothstep(0.5, 0.0, length(n + vec2f(0.30, 0.32)));
  let light = (dome * 0.05 + highlight * 0.16) * lightScale;
  result = result * shade + vec3f(light);
  return mix(pixel, result, mask);
}

// rainStaticLayer scans the 3x3 cell neighborhood so droplets centered near
// a cell border stay whole instead of being sliced at the boundary. The
// twinkle flag gives the condensation layer a slow glint.
fn rainStaticLayer(uv: vec2f, pp: vec2f, pixel: vec3f, density: f32,
  cellSize: f32, seed: u32, time: f32, refraction: f32, opacity: f32,
  twinkle: bool, dims: vec2f) -> vec3f {
  let baseCell = floor(pp / cellSize);
  var color = pixel;
  for (var j = -1; j <= 1; j++) {
    for (var i = -1; i <= 1; i++) {
      let cell = baseCell + vec2f(f32(i), f32(j));
      if (cell.x < 0.0 || cell.y < 0.0) {
        continue;
      }
      let h = rainHash(cell.x, cell.y, seed);
      if (h.x >= density) {
        continue;
      }
      var alpha = opacity;
      if (twinkle) {
        alpha *= 0.85 + 0.15 * sin(time * (1.0 + 2.0 * h.z) + h.x * 6.2832);
      }
      let center = (cell + h.yz) * cellSize;
      let radius = cellSize * (0.30 + 0.15 * h.x);
      color = rainDrop(uv, pp, color, center, vec2f(radius), refraction, alpha, 1.0,
        fract(h.x * 61.8034), dims);
    }
  }
  return color;
}

// rainErodeLayer is the splash layer: each cell's droplet appears, holds,
// then gradually shrinks away (alpha erosion). The position hash is re-keyed
// with the lifecycle index, so every cycle lands somewhere new in the cell.
fn rainErodeLayer(uv: vec2f, pp: vec2f, pixel: vec3f, density: f32,
  cellSize: f32, speed: f32, seed: u32, time: f32, refraction: f32,
  opacity: f32, dims: vec2f) -> vec3f {
  let baseCell = floor(pp / cellSize);
  var color = pixel;
  for (var j = -1; j <= 1; j++) {
    for (var i = -1; i <= 1; i++) {
      let cell = baseCell + vec2f(f32(i), f32(j));
      if (cell.x < 0.0 || cell.y < 0.0) {
        continue;
      }
      let h = rainHash(cell.x, cell.y, seed);
      if (h.x >= density) {
        continue;
      }
      let tCycle = time * speed * (0.6 + 0.8 * h.y) + h.x;
      let phase = fract(tCycle);
      let grow = smoothstep(0.0, 0.07, phase);
      let shrink = 1.0 - smoothstep(0.40, 0.97, phase);
      let radiusScale = grow * shrink;
      if (radiusScale <= 0.0) {
        continue;
      }
      let hp = rainHash(cell.x, cell.y, seed + u32(floor(tCycle)));
      let radius = cellSize * (0.30 + 0.15 * hp.x) * radiusScale;
      if (radius < 0.5) {
        continue;
      }
      let center = (cell + hp.yz) * cellSize;
      color = rainDrop(uv, pp, color, center, vec2f(radius), refraction,
        opacity * shrink, 0.3 + 0.7 * shrink, fract(hp.x * 61.8034), dims);
    }
  }
  return color;
}

@fragment
fn fs_main(in: VertexOut) -> @location(0) vec4f {
  let source = textureSampleLevel(srcTexture, srcSampler, in.uv, 0.0);
  let density = clamp(params.values.x, 0.0, 1.0);
  if (density == 0.0) {
    return source;
  }
  let size = select(48.0, params.values.y, params.values.y > 0.0);
  let speed = select(0.15, params.values.z, params.values.z != 0.0);
  let dims = params.state.yz;
  let pp = in.uv * dims;

  var color = source.rgb;
  color = rainStaticLayer(in.uv, pp, color, density, size * 0.25,
    params.seed + 0x13u, params.state.x, params.values.w * 0.45, 0.95, true, dims);
  color = rainStaticLayer(in.uv, pp, color, density * 0.55, size * 0.85,
    params.seed + 0x29u, params.state.x, params.values.w * 0.80, 1.0, false, dims);
  color = rainErodeLayer(in.uv, pp, color, density * 0.50, size * 2.20, speed,
    params.seed + 0x41u, params.state.x, params.values.w * 1.25, 1.0, dims);
  return vec4f(color, source.a);
}
