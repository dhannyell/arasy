// Mirrors platform/raster/postfx.go applyBloom's bright and Kawase passes.
struct VertexOut {
  @builtin(position) position: vec4f,
  @location(0) uv: vec2f,
};

struct Params {
  values: vec4f,
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

fn sampleAt(pixel: vec2f, dimensions: vec2f) -> vec4f {
  return textureSampleLevel(srcTexture, srcSampler, (pixel + vec2f(0.5)) / dimensions, 0.0);
}

@fragment
fn fs_bright(in: VertexOut) -> @location(0) vec4f {
  let source = textureSampleLevel(srcTexture, srcSampler, in.uv, 0.0);
  return vec4f(max(source.rgb - vec3f(params.values.x), vec3f(0.0)), source.a);
}

@fragment
fn fs_down(in: VertexOut) -> @location(0) vec4f {
  let sourceDimensions = params.values.xy;
  let center = in.uv * sourceDimensions - vec2f(0.5);
  var sum = sampleAt(center, sourceDimensions) * 4.0;
  sum += sampleAt(center + vec2f(-0.5, -0.5), sourceDimensions);
  sum += sampleAt(center + vec2f(0.5, -0.5), sourceDimensions);
  sum += sampleAt(center + vec2f(-0.5, 0.5), sourceDimensions);
  sum += sampleAt(center + vec2f(0.5, 0.5), sourceDimensions);
  return sum / 8.0;
}

@fragment
fn fs_up(in: VertexOut) -> @location(0) vec4f {
  let sourceDimensions = params.values.xy;
  let center = in.uv * sourceDimensions - vec2f(0.5);
  var sum = sampleAt(center + vec2f(-1.0, 0.0), sourceDimensions) * 2.0;
  sum += sampleAt(center + vec2f(1.0, 0.0), sourceDimensions) * 2.0;
  sum += sampleAt(center + vec2f(0.0, -1.0), sourceDimensions) * 2.0;
  sum += sampleAt(center + vec2f(0.0, 1.0), sourceDimensions) * 2.0;
  sum += sampleAt(center + vec2f(-1.0, -1.0), sourceDimensions);
  sum += sampleAt(center + vec2f(1.0, -1.0), sourceDimensions);
  sum += sampleAt(center + vec2f(-1.0, 1.0), sourceDimensions);
  sum += sampleAt(center + vec2f(1.0, 1.0), sourceDimensions);
  return sum / 12.0;
}
