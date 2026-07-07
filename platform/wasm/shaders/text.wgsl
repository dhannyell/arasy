struct VertexOut {
  @builtin(position) position: vec4f,
  @location(0) color: vec4f,
  @location(1) uv: vec2f,
};

@group(0) @binding(0) var atlasTexture: texture_2d<f32>;
@group(0) @binding(1) var atlasSampler: sampler;

@vertex
fn vs_main(
  @location(0) position: vec2f,
  @location(1) color: vec4f,
  @location(2) uv: vec2f,
) -> VertexOut {
  var out: VertexOut;
  out.position = vec4f(position, 0.0, 1.0);
  out.color = color;
  out.uv = uv;
  return out;
}

@fragment
fn fs_main(in: VertexOut) -> @location(0) vec4f {
  let mask = textureSample(atlasTexture, atlasSampler, in.uv).r;
  return vec4f(in.color.rgb, in.color.a * mask);
}
