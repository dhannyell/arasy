// Retained animated sprite batch, single texture. Same 64-byte base record
// as the streaming sprite shader, plus a per-instance animation record and a
// per-draw uniform block (dynamic offset). Flip-book frame selection and
// bounce/pulse mirror internal/renderbatch's canonical evaluation: the CPU
// wraps time into [0, 4096) before packing it into the uniform.

struct Sprite {
  center : vec2f,
  half_size : vec2f,
  rotation : f32,
  flags : u32,
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

struct BatchUniforms {
	world_to_clip_x : vec2f,
	world_to_clip_y : vec2f,
	to_clip_offset : vec2f,
	world_to_screen_scale : vec2f,
	screen_to_clip_scale : vec2f,
	_camera_pad : vec2f,
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
const PROJECT_CORNERS = 1u;

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

  let rotation = vec2f(cos(sprite.rotation), sin(sprite.rotation));
	let local_corner = corner * sprite.half_size;
  let rotated_world_corner = vec2f(
	    local_corner.x * rotation.x - local_corner.y * rotation.y,
	    local_corner.x * rotation.y + local_corner.y * rotation.x,
  );
  var clip_pos : vec2f;
  if ((sprite.flags & PROJECT_CORNERS) != 0u) {
    let world_corner = sprite.center + rotated_world_corner;
    clip_pos = u.world_to_clip_x * world_corner.x +
      u.world_to_clip_y * world_corner.y + u.to_clip_offset;
  } else {
    // Preserve the upright-sprite rule: scale into screen pixels before
    // rotating, so AspectStretch does not skew a rotated sprite.
    let screen_corner = local_corner * u.world_to_screen_scale;
    let rotated_screen_corner = vec2f(
      screen_corner.x * rotation.x - screen_corner.y * rotation.y,
      screen_corner.x * rotation.y + screen_corner.y * rotation.x,
    );
    let world_center = u.world_to_clip_x * sprite.center.x +
      u.world_to_clip_y * sprite.center.y;
    clip_pos = world_center + u.to_clip_offset +
      rotated_screen_corner * u.screen_to_clip_scale;
  }

  var out : VSOut;
  out.position = vec4f(clip_pos, 0.0, 1.0);
  out.uv = mix(uv_min, uv_max, corner * 0.5 + vec2f(0.5));
  out.tint = unpack4x8unorm(sprite.tint);
  out.screen_pos = vec2f(
		(clip_pos.x + 1.0) / u.screen_to_clip_scale.x,
		(clip_pos.y - 1.0) / u.screen_to_clip_scale.y,
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
