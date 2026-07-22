// Image/texture pipeline. Blend is premultiplied (see webgpu.go's
// premultipliedBlend): asset/pixel decodes textures into premultiplied-alpha
// pixels and tess premultiplies the tint, unlike the solid pipeline which
// stays straight-alpha.
struct VertexOut {
  @builtin(position) position: vec4f,
  @location(0) color: vec4f,
  @location(1) uv: vec2f,
};

@group(0) @binding(0) var tex: texture_2d<f32>;
@group(0) @binding(1) var samp: sampler;

@vertex
fn vs_main(
  @location(0) position: vec4f,
  @location(1) color: vec4f,
  @location(2) uv: vec2f,
) -> VertexOut {
  var out: VertexOut;
  // position is already xw, yw, 0, w: the hardware perspective divide and
  // UV interpolation across the primitive are perspective-correct.
  out.position = position;
  out.color = color;
  out.uv = uv;
  return out;
}

@fragment
fn fs_main(in: VertexOut) -> @location(0) vec4f {
  return textureSample(tex, samp, in.uv) * in.color;
}
