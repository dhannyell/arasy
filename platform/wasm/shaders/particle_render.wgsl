struct Particle {
  pos : vec2f, vel : vec2f,
  age : f32, life : f32,
  size0 : f32, size1 : f32,
  spin : f32, counter : u32,
  wobble_axis : vec2f,
}

struct EmitterUniforms {
  color_start : vec4f, color_end : vec4f,
  to_clip_scale : vec2f, to_clip_offset : vec2f,
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
@group(0) @binding(1) var<storage, read> particles : array<Particle>;
@group(0) @binding(2) var<storage, read> alive_list : array<u32>;
@group(1) @binding(0) var tex : texture_2d<f32>;
@group(1) @binding(1) var samp : sampler;

// mirror: particle.pcg3d (hash.go); duplicated per module because compute
// and render declare the same buffers in different address spaces.
fn pcg3d(p : vec3u) -> vec3u {
  var v = p * 1664525u + 1013904223u;
  v.x += v.y * v.z;  v.y += v.z * v.x;  v.z += v.x * v.y;
  v ^= v >> vec3u(16u);
  v.x += v.y * v.z;  v.y += v.z * v.x;  v.z += v.x * v.y;
  return v;
}
fn float01(x : u32) -> f32 { return f32(x) * 2.3283064365386963e-10; }

const TAU = 6.28318530717958647;

const CORNERS = array<vec2f, 6>(
  vec2f(-1.0, -1.0), vec2f(1.0, -1.0), vec2f(1.0, 1.0),
  vec2f(-1.0, -1.0), vec2f(1.0, 1.0),  vec2f(-1.0, 1.0),
);

struct VSOut {
  @builtin(position) pos : vec4f,
  @location(0) color : vec4f,
  @location(1) uv : vec2f,
}

@vertex
fn vs_main(@builtin(vertex_index) vi : u32,
           @builtin(instance_index) ii : u32) -> VSOut {
  let p = particles[alive_list[ii]];
  let progress = clamp(p.age / p.life, 0.0, 1.0);

  // Re-derive h4 from the stored spawn counter: initial rotation + wobble
  // phase (mirror: spawn lane map) without growing the particle struct.
  let h4 = pcg3d(pcg3d(pcg3d(pcg3d(vec3u(p.counter, u.seed_lo, u.seed_hi)))));
  let rot = mix(u.rot_min, u.rot_max, float01(h4.x)) + p.spin * p.age;

  var center = p.pos;
  if (u.wobble_amp != 0.0 && u.wobble_freq != 0.0) {
    // mirror: Emitter.Alive wobble — draw-time only, never integrated
    let phase = TAU * u.wobble_freq * p.age + TAU * float01(h4.y);
    center += p.wobble_axis * (u.wobble_amp * sin(phase));
  }

  let half = mix(p.size0, p.size1, progress) * 0.5;
  var flip_x = 1.0;
  if (u.flip_x_freq != 0.0) {
    flip_x = cos(TAU * u.flip_x_freq * p.age + TAU * float01(h4.y));
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
  let center_clip = center * u.to_clip_scale + u.to_clip_offset;
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

@fragment
fn fs_main(in : VSOut) -> @location(0) vec4f {
  return textureSample(tex, samp, in.uv) * in.color;
}

// Missing textures are diagnostic geometry, not effect content. Keep the
// placeholder visibly magenta regardless of the particle's tint/alpha.
@fragment
fn fs_missing(in : VSOut) -> @location(0) vec4f {
  return vec4f(1.0, 0.0, 1.0, 1.0);
}
