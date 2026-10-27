struct VertexOut {
    @builtin(position) position: vec4f,
    @location(0) uv: vec2f,
};

// lut holds the cell's ordered-dither thresholds row-major, four per vec4f,
// for a cell of state.w pixels a side. noise is the threshold jitter
// amplitude and seed the host's tick-folded key, as in post_grain.wgsl.
struct Params {
    values: vec4f,
    state: vec4f,
    noise: f32,
    seed: u32,
    pad: vec2f,
    pad2: vec4f,
    lut: array<vec4f, 64>,
};

@group(0) @binding(0) var srcTexture: texture_2d<f32>;
@group(0) @binding(1) var srcSampler: sampler;
@group(0) @binding(2) var<uniform> params: Params;

// mirror: particle.pcg3d (hash.go), same as post_grain.wgsl.
fn pcg3d(p: vec3u) -> vec3u {
    var v = p * 1664525u + 1013904223u;
    v.x += v.y * v.z;
    v.y += v.z * v.x;
    v.z += v.x * v.y;
    v ^= v >> vec3u(16u);
    v.x += v.y * v.z;
    v.y += v.z * v.x;
    v.z += v.x * v.y;
    return v;
}

fn float01(value: u32) -> f32 {
    return f32(value) * 2.3283064365386963e-10;
}

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
    let strength = clamp(params.values.w, 0.0, 1.0);
    if (strength == 0.0) {
        return source;
    }
    let levels = max(params.values.y, 2.0);
    let steps = levels - 1.0;

    let dims = params.state.yz;
    let cell = u32(params.state.w);
    let pixel = max(params.state.x, 1.0);
    let grid = vec2u(floor(floor(in.uv * dims) / pixel));
    let px = grid % cell;
    let i = px.y * cell + px.x;
    var m = params.lut[i >> 2u][i & 3u];
    if (params.noise != 0.0) {
        let hash = pcg3d(vec3u(grid.x, grid.y, params.seed));
        m += (float01(hash.x) - 0.5) * params.noise;
    }

    let luma = dot(source.rgb, vec3f(0.2126, 0.7152, 0.0722));
    let toned = clamp(round(luma * steps + m - 0.5) / steps, 0.0, 1.0);

    // Adding the tone change to every channel keeps the dither one step deep
    // whatever the colour; scaling by toned/luma would amplify it by 1/luma.
    let rgb = source.rgb + vec3f((toned - luma) * strength);
    return vec4f(rgb, source.a);
}