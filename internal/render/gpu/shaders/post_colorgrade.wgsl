// Mirrors platform/raster/postfx.go applyColorGrade.
struct VertexOut {
  @builtin(position) position: vec4f,
  @location(0) uv: vec2f,
};

struct Params {
  grade: vec4f,
  tint: vec4f,
};

@group(0) @binding(0) var srcTexture: texture_2d<f32>;
@group(0) @binding(1) var srcSampler: sampler;
@group(0) @binding(2) var<uniform> params: Params;

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
  let source = textureSampleLevel(srcTexture, srcSampler, in.uv, 0.0);
  let contrast = 1.0 + params.grade.y;
  let saturation = 1.0 + params.grade.z;
  var rgb = (source.rgb - vec3f(0.5)) * contrast + vec3f(0.5) + params.grade.x;
  let luma = dot(rgb, vec3f(0.2126, 0.7152, 0.0722));
  rgb = vec3f(luma) + (rgb - vec3f(luma)) * saturation;
  rgb *= params.tint.rgb;
  return vec4f(rgb, source.a);
}
