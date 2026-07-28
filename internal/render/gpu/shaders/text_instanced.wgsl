struct Glyph {
  pen_x : f32,
  metric : u32,
  label : u32,
  _pad : u32,
}

struct GlyphMetric {
  uv_min : vec2f,
  uv_max : vec2f,
  size : vec2f,
  offset : vec2f,
}

struct Label {
  position : vec2f,
  transform_ab : vec2f,
  transform_cd : vec2f,
  origin_baseline : vec2f,
  clip_min : vec2f,
  clip_max : vec2f,
  scale : f32,
  tint : u32,
  _pad : vec2u,
}

struct Camera {
  to_clip_scale : vec2f,
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
}

@group(0) @binding(0) var atlas_texture : texture_2d<f32>;
@group(0) @binding(1) var atlas_sampler : sampler;
@group(0) @binding(2) var<storage, read> glyphs : array<Glyph>;
@group(0) @binding(3) var<storage, read> labels : array<Label>;
@group(0) @binding(4) var<storage, read> metrics : array<GlyphMetric>;
@group(0) @binding(5) var<uniform> camera : Camera;

const CORNERS = array<vec2f, 6>(
  vec2f(-1.0, -1.0), vec2f(1.0, -1.0), vec2f(1.0, 1.0),
  vec2f(-1.0, -1.0), vec2f(1.0, 1.0), vec2f(-1.0, 1.0),
);

@vertex
fn vs_main(@builtin(vertex_index) vertex_index : u32) -> VSOut {
  let glyph = glyphs[vertex_index / 6u];
  let label = labels[glyph.label];
  let metric = metrics[glyph.metric];
  let corner = CORNERS[vertex_index % 6u];
  let half_size = metric.size * label.scale * 0.5;
  let center = vec2f(
    label.origin_baseline.x + glyph.pen_x + (metric.offset.x + metric.size.x * 0.5) * label.scale,
    label.origin_baseline.y + (metric.offset.y + metric.size.y * 0.5) * label.scale,
  );
  let local = center + corner * half_size;
  let transformed = vec2f(
    local.x * label.transform_ab.x + local.y * label.transform_cd.x,
    local.x * label.transform_ab.y + local.y * label.transform_cd.y,
  );
  let screen_pos = label.position + transformed;
  let clip_pos = vec2f(
    screen_pos.x * camera.screen_to_clip_scale.x - 1.0,
    screen_pos.y * camera.screen_to_clip_scale.y + 1.0,
  );

  var out : VSOut;
  out.position = vec4f(clip_pos, 0.0, 1.0);
  out.uv = mix(metric.uv_min, metric.uv_max, corner * 0.5 + vec2f(0.5));
  out.tint = unpack4x8unorm(label.tint);
  out.screen_pos = screen_pos;
  out.clip_min = label.clip_min;
  out.clip_max = label.clip_max;
  return out;
}

@fragment
fn fs_main(in : VSOut) -> @location(0) vec4f {
  if (in.screen_pos.x < in.clip_min.x || in.screen_pos.x > in.clip_max.x ||
      in.screen_pos.y < in.clip_min.y || in.screen_pos.y > in.clip_max.y) {
    discard;
  }
  let mask = textureSample(atlas_texture, atlas_sampler, in.uv).r;
  return vec4f(in.tint.rgb, in.tint.a * mask);
}
