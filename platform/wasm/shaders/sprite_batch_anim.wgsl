// Retained animated sprite batch, single texture. Same 64-byte base record
// as the streaming sprite shader, plus a per-instance animation record and a
// per-draw uniform block (dynamic offset). Flip-book frame selection and
// bounce/pulse mirror internal/renderbatch's canonical evaluation: the CPU
// wraps time into [0, 4096) before packing it into the uniform.

struct Sprite {
  center : vec2f,
  half_size : vec2f,
  rotation : f32,
  _pad : f32,
  uv_min : vec2f,
  uv_max : vec2f,
  clip_min : vec2f,
  clip_max : vec2f,
  tint : u32,
  texture_layer : u32,
}

struct SpriteAnim {
  frames : u32,
  cells_per_row : u32,
  base_index : u32,
  _pad0 : u32,
  rate : f32,
  phase : f32,
  bounce : f32,
  _pad1 : f32,
}

struct CameraUniforms {
  to_clip_scale : vec2f,
  to_clip_offset : vec2f,
  world_to_screen_scale : vec2f,
  screen_to_clip_scale : vec2f,
}

struct BatchUniforms {
  camera : CameraUniforms,
  time : f32,
  bounce_hz : f32,
  intensity : f32,
  _pad0 : f32,
  pulse_center : vec2f,
  pulse_radius : f32,
  pulse_width : f32,
  pulse_boost : f32,
  _pad1 : f32,
  _pad2 : f32,
  _pad3 : f32,
}

struct VSOut {
  @builtin(position) position : vec4f,
  @location(0) uv : vec2f,
  @location(1) tint : vec4f,
  @location(2) screen_pos : vec2f,
  @location(3) clip_min : vec2f,
  @location(4) clip_max : vec2f,
}

@group(0) @binding(0) var tex : texture_2d<f32>;
@group(0) @binding(1) var samp : sampler;
@group(0) @binding(2) var<storage, read> sprites : array<Sprite>;
@group(0) @binding(3) var<uniform> u : BatchUniforms;
@group(0) @binding(4) var<storage, read> anims : array<SpriteAnim>;

const CORNERS = array<vec2f, 6>(
  vec2f(-1.0, -1.0), vec2f(1.0, -1.0), vec2f(1.0, 1.0),
  vec2f(-1.0, -1.0), vec2f(1.0, 1.0), vec2f(-1.0, 1.0),
);

const TWO_PI = 6.283185307179586;

@vertex
fn vs_main(@builtin(vertex_index) vertex_index : u32) -> VSOut {
  var sprite = sprites[vertex_index / 6u];
  let anim = anims[vertex_index / 6u];
  let corner = CORNERS[vertex_index % 6u];

  let phase = anim.phase - floor(anim.phase);

  var uv_min = sprite.uv_min;
  var uv_max = sprite.uv_max;
  if (anim.frames > 1u && anim.rate > 0.0) {
    // Row-major flip-book advance from the base cell, mirroring
    // renderbatch.AnimatedSrc's frame selection exactly.
    let step = floor(u.time * anim.rate + phase * f32(anim.frames));
    let frame = u32(step) % anim.frames;
    let cell = anim.base_index + frame;
    let delta_col = f32(cell % anim.cells_per_row) - f32(anim.base_index % anim.cells_per_row);
    let delta_row = f32(cell / anim.cells_per_row) - f32(anim.base_index / anim.cells_per_row);
    let duv = (uv_max - uv_min) * vec2f(delta_col, delta_row);
    uv_min = uv_min + duv;
    uv_max = uv_max + duv;
  }

  if (u.bounce_hz > 0.0 && u.intensity != 0.0 && anim.bounce != 0.0) {
    let s = max(0.0, sin(TWO_PI * (u.time * u.bounce_hz + phase)));
    var boost = 1.0;
    if (u.pulse_width > 0.0 && u.pulse_boost != 0.0) {
      let d = abs(distance(sprite.center, u.pulse_center) - u.pulse_radius);
      boost = 1.0 + u.pulse_boost * exp(-(d * d) / (2.0 * u.pulse_width * u.pulse_width));
    }
    sprite.center.y = sprite.center.y - anim.bounce * u.intensity * boost * s;
  }

  // Match the legacy DrawImage path: size is scaled into screen pixels
  // before rotation so AspectStretch cannot skew the rotated sprite.
  let rotation = vec2f(cos(sprite.rotation), sin(sprite.rotation));
  let screen_corner = corner * sprite.half_size * u.camera.world_to_screen_scale;
  let rotated = vec2f(
    screen_corner.x * rotation.x - screen_corner.y * rotation.y,
    screen_corner.x * rotation.y + screen_corner.y * rotation.x,
  );
  let clip_pos = sprite.center * u.camera.to_clip_scale + u.camera.to_clip_offset +
    rotated * u.camera.screen_to_clip_scale;

  var out : VSOut;
  out.position = vec4f(clip_pos, 0.0, 1.0);
  out.uv = mix(uv_min, uv_max, corner * 0.5 + vec2f(0.5));
  out.tint = unpack4x8unorm(sprite.tint);
  out.screen_pos = vec2f(
    (clip_pos.x + 1.0) / u.camera.screen_to_clip_scale.x,
    (clip_pos.y - 1.0) / u.camera.screen_to_clip_scale.y,
  );
  out.clip_min = sprite.clip_min;
  out.clip_max = sprite.clip_max;
  return out;
}

@fragment
fn fs_main(in : VSOut) -> @location(0) vec4f {
  if (in.screen_pos.x < in.clip_min.x || in.screen_pos.x > in.clip_max.x ||
      in.screen_pos.y < in.clip_min.y || in.screen_pos.y > in.clip_max.y) {
    discard;
  }
  return textureSample(tex, samp, in.uv) * in.tint;
}
