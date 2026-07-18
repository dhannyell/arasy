// Fullscreen composite of one particle offscreen target into the main pass.
// Both passes share the same NDC convention, so the mapping is an identity:
// NDC (-1,-1) is the framebuffer's bottom-left, which is uv (0,1).

@group(0) @binding(0) var tex : texture_2d<f32>;
@group(0) @binding(1) var samp : sampler;

struct VSOut {
  @builtin(position) pos : vec4f,
  @location(0) uv : vec2f,
}

@vertex
fn vs_main(@builtin(vertex_index) vi : u32) -> VSOut {
  let xy = vec2f(f32((vi << 1u) & 2u), f32(vi & 2u));
  var out : VSOut;
  out.pos = vec4f(xy * 2.0 - 1.0, 0.0, 1.0);
  out.uv = vec2f(xy.x, 1.0 - xy.y);
  return out;
}

// The offscreen holds premultiplied color; the pipeline blends it source-over
// so the composite is exact for alpha particles and — because additive
// particles accumulate with zero destination-alpha contribution — exact for
// additive ones too.
@fragment
fn fs_main(in : VSOut) -> @location(0) vec4f {
  return textureSample(tex, samp, in.uv);
}
