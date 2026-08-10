// Mirrors the hot/cold split in particle_compute.wgsl: hot is the simulated
// state, cold is written once at spawn.
struct ParticleHot {
  pos : vec2f, vel : vec2f,
  age : f32, life : f32,
}

struct ParticleCold {
  size0 : f32, size1 : f32,
  spin : f32, rot0 : f32,
  wobble_axis : vec2f,
  phase : f32,
}

struct EmitterUniforms {
  color_start : vec4f, color_end : vec4f,
  world_to_clip_x : vec2f, world_to_clip_y : vec2f,
  to_clip_offset : vec2f,
  world_to_screen_scale : vec2f,
  screen_to_clip_scale : vec2f,
  pos : vec2f, prev_pos : vec2f,
  gravity : vec2f, emit_extent : vec2f, emit_scale : vec2f, wobble_axis : vec2f,
  dt : f32, drag : f32,
  emit_count : u32, emit_base : u32, seed_lo : u32, seed_hi : u32,
  dir_base : f32, spread : f32,
  speed_min : f32, speed_max : f32, life_min : f32, life_max : f32,
  size0_min : f32, size0_max : f32, size1_min : f32, size1_max : f32,
  spin_max : f32, rot_min : f32, rot_max : f32,
  wobble_amp : f32, wobble_freq : f32, fade_in : f32, flip_x_freq : f32,
  shape : u32, vel_mode : u32, emit_radius : f32, capacity : u32,
}

@group(0) @binding(0) var<uniform> u : EmitterUniforms;
@group(0) @binding(1) var<storage, read> hot : array<ParticleHot>;
@group(0) @binding(2) var<storage, read> alive_list : array<u32>;
@group(0) @binding(3) var<storage, read> cold : array<ParticleCold>;
@group(1) @binding(0) var tex : texture_2d<f32>;
@group(1) @binding(1) var samp : sampler;

const TAU = 6.28318530717958647;

// One oversized triangle circumscribes the unit quad: half the raster setup
// of a two-triangle quad, which measurement showed is the residual draw cost
// for pixel-scale particles. The fragment stage masks the overhang back to
// the quad with an analytically anti-aliased edge.
const CORNERS = array<vec2f, 3>(
  vec2f(-1.0, -1.0), vec2f(3.0, -1.0), vec2f(-1.0, 3.0),
);

struct VSOut {
  @builtin(position) pos : vec4f,
  @location(0) color : vec4f,
  @location(1) uv : vec2f,
}

@vertex
fn vs_main(@builtin(vertex_index) vi : u32,
           @builtin(instance_index) ii : u32) -> VSOut {
  let slot = alive_list[ii];
  let p = hot[slot];
  let spawn = cold[slot];
  let progress = clamp(p.age / p.life, 0.0, 1.0);

  // rot0 and phase were derived from the h4 spawn lanes in the emit pass
  // (mirror: spawn lane map), so no hashing happens per vertex.
  let rot = spawn.rot0 + spawn.spin * p.age;

  var center = p.pos;
  if (u.wobble_amp != 0.0 && u.wobble_freq != 0.0) {
    // mirror: Emitter.Alive wobble — draw-time only, never integrated
    let phase = TAU * u.wobble_freq * p.age + TAU * spawn.phase;
    center += spawn.wobble_axis * (u.wobble_amp * sin(phase));
  }

  let half = mix(spawn.size0, spawn.size1, progress) * 0.5;
  var flip_x = 1.0;
  if (u.flip_x_freq != 0.0) {
    flip_x = cos(TAU * u.flip_x_freq * p.age + TAU * spawn.phase);
  }
  let corner = CORNERS[vi] * vec2f(half * flip_x, half);
  let c = cos(rot);
  let s = sin(rot);

  // Match DrawImage: map the axis-aligned particle size to screen pixels,
  // then rotate around its screen-space center. Rotating in world space
  // first would skew the quad under AspectStretch's non-uniform scale.
  let screen_corner = corner * u.world_to_screen_scale;
  let rotated_screen = vec2f(screen_corner.x * c - screen_corner.y * s,
                             screen_corner.x * s + screen_corner.y * c);
  let center_clip = center.x * u.world_to_clip_x +
                    center.y * u.world_to_clip_y + u.to_clip_offset;
  let local_clip = rotated_screen * u.screen_to_clip_scale;

  var color = mix(u.color_start, u.color_end, progress);
  if (u.fade_in > 0.0) {
    color.a *= min(1.0, progress / u.fade_in);
  }

  var out : VSOut;
  out.pos = vec4f(center_clip + local_clip, 0.0, 1.0);
  out.color = vec4f(color.rgb * color.a, color.a); // premultiplied
  out.uv = CORNERS[vi] * 0.5 + vec2f(0.5);
  return out;
}

// quad_coverage masks the circumscribing triangle down to the particle quad
// with an analytically anti-aliased edge about one pixel wide: per-axis
// distance to the nearest quad edge in uv space, converted to pixel coverage
// through the fragment's uv footprint.
fn quad_coverage(uv : vec2f) -> f32 {
  let fw = max(fwidth(uv), vec2f(1e-6));
  let d = min(uv, vec2f(1.0) - uv);
  let cov = clamp(d / fw + vec2f(0.5), vec2f(0.0), vec2f(1.0));
  return cov.x * cov.y;
}

@fragment
fn fs_main(in : VSOut) -> @location(0) vec4f {
  // Color is premultiplied, so scaling the whole vector by coverage is the
  // correct edge attenuation for both source-over and additive blends.
  return textureSample(tex, samp, in.uv) * in.color * quad_coverage(in.uv);
}

// Missing textures are diagnostic geometry, not effect content. Keep the
// placeholder visibly magenta regardless of the particle's tint/alpha.
@fragment
fn fs_missing(in : VSOut) -> @location(0) vec4f {
  return vec4f(1.0, 0.0, 1.0, 1.0) * quad_coverage(in.uv);
}
