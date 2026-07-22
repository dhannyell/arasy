// Internal mip-chain generation shader: not a rendering pipeline exposed to
// game content, only a boot-time helper invoked once per declared texture.
//
// The vertex shader draws a single fullscreen triangle from vertexIndex
// alone (no vertex buffer): it covers the whole clip space so the fragment
// shader runs once per destination pixel, downsampling the previous mip
// level through a linear sampler. One render pass per mip level, source and
// destination views pointing at adjacent levels of the same texture.
struct VertexOut {
  @builtin(position) position: vec4f,
  @location(0) uv: vec2f,
};

@group(0) @binding(0) var srcTexture: texture_2d<f32>;
@group(0) @binding(1) var srcSampler: sampler;

@vertex
fn vs_main(@builtin(vertex_index) vertexIndex: u32) -> VertexOut {
  var out: VertexOut;
  let x = f32((vertexIndex << 1u) & 2u);
  let y = f32(vertexIndex & 2u);
  out.uv = vec2f(x, y);
  out.position = vec4f(x * 2.0 - 1.0, 1.0 - y * 2.0, 0.0, 1.0);
  return out;
}

@fragment
fn fs_main(in: VertexOut) -> @location(0) vec4f {
  return textureSample(srcTexture, srcSampler, in.uv);
}
