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

struct SpriteUniforms {
	world_to_clip_x : vec2f,
	world_to_clip_y : vec2f,
	to_clip_offset : vec2f,
	world_to_screen_scale : vec2f,
	screen_to_clip_scale : vec2f,
}

struct VSOut {
  @builtin(position) position : vec4f,
  @location(0) uv : vec2f,
  @location(1) tint : vec4f,
  @location(2) screen_pos : vec2f,
  @location(3) clip_min : vec2f,
  @location(4) clip_max : vec2f,
  @location(5) @interpolate(flat) texture_layer : u32,
}

@group(0) @binding(0) var tex : texture_2d_array<f32>;
@group(0) @binding(1) var samp : sampler;
@group(0) @binding(2) var<storage, read> sprites : array<Sprite>;
@group(0) @binding(3) var<uniform> camera : SpriteUniforms;

const CORNERS = array<vec2f, 6>(
  vec2f(-1.0, -1.0), vec2f(1.0, -1.0), vec2f(1.0, 1.0),
  vec2f(-1.0, -1.0), vec2f(1.0, 1.0), vec2f(-1.0, 1.0),
);

@vertex
fn vs_main(@builtin(vertex_index) vertex_index : u32) -> VSOut {
  let sprite = sprites[vertex_index / 6u];
  let corner = CORNERS[vertex_index % 6u];
  let rotation = vec2f(cos(sprite.rotation), sin(sprite.rotation));
  let screen_corner = corner * sprite.half_size * camera.world_to_screen_scale;
  let rotated = vec2f(
    screen_corner.x * rotation.x - screen_corner.y * rotation.y,
    screen_corner.x * rotation.y + screen_corner.y * rotation.x,
  );
	let world_center = camera.world_to_clip_x * sprite.center.x +
		camera.world_to_clip_y * sprite.center.y;
	let clip_pos = world_center + camera.to_clip_offset +
		rotated * camera.screen_to_clip_scale;

  var out : VSOut;
  out.position = vec4f(clip_pos, 0.0, 1.0);
  out.uv = mix(sprite.uv_min, sprite.uv_max, corner * 0.5 + vec2f(0.5));
  out.tint = unpack4x8unorm(sprite.tint);
  out.screen_pos = vec2f(
    (clip_pos.x + 1.0) / camera.screen_to_clip_scale.x,
    (clip_pos.y - 1.0) / camera.screen_to_clip_scale.y,
  );
  out.clip_min = sprite.clip_min;
  out.clip_max = sprite.clip_max;
  out.texture_layer = sprite.texture_layer;
  return out;
}

@fragment
fn fs_main(in : VSOut) -> @location(0) vec4f {
  if (in.screen_pos.x < in.clip_min.x || in.screen_pos.x > in.clip_max.x ||
      in.screen_pos.y < in.clip_min.y || in.screen_pos.y > in.clip_max.y) {
    discard;
  }
  return textureSample(tex, samp, in.uv, i32(in.texture_layer)) * in.tint;
}
