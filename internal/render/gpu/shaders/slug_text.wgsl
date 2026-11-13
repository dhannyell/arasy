// Draws DrawText layers from their outlines. Add it after slug.CoreWGSL.
// The output is straight alpha, so the host uses the same blends as atlas text.

struct Camera {
    parallax_clip_offset: vec2f,
    _pad_world_to_clip_y: vec2f,
    _pad_to_clip_offset: vec2f,
    _pad_world_to_screen_scale: vec2f,
    screen_to_clip_scale: vec2f,
}

// One layer of one glyph. tess.SlugInstanceStride is its size.
struct Instance {
    linear: vec4f, // Affine a, b, c, d: em space to screen pixels
    color: vec4f, // straight RGB and alpha of the command
    translate: vec2f, // Affine e, f
    clip_min: vec2f,
    clip_max: vec2f,
    layer: u32,
    _pad: u32,
}

@group(0) @binding(0) var<uniform> camera: Camera;
@group(0) @binding(1) var<storage, read> slug_curves: array<vec4f>;
@group(0) @binding(2) var<storage, read> slug_bands: array<u32>;
@group(0) @binding(3) var<storage, read> slug_layers: array<u32>;
@group(0) @binding(4) var<storage, read> instances: array<Instance>;

fn slug_curve(texel: u32) -> vec4f { return slug_curves[texel]; }
fn slug_band(word: u32) -> u32 { return slug_bands[word]; }
fn slug_layer(word: u32) -> u32 { return slug_layers[word]; }

struct Varying {
    @builtin(position) position: vec4f,
    @location(0) coord: vec2f,
    @location(1) screen: vec2f,
    @location(2) color: vec4f,
    @location(3) @interpolate(flat) clip: vec4f,
    @location(4) @interpolate(flat) layer: u32,
}

@vertex
fn vs_main(@builtin(vertex_index) vertex: u32) -> Varying {
    const CORNERS = array(
        vec2f(-1.0, -1.0), vec2f(1.0, -1.0), vec2f(1.0, 1.0),
        vec2f(-1.0, -1.0), vec2f(1.0, 1.0), vec2f(-1.0, 1.0),
    );
    let inst = instances[vertex / 6u];
    let corner = CORNERS[vertex % 6u];
    let l = slug_layer_at(inst.layer);
    let lo = l.bounds.xy;
    let hi = l.bounds.zw;
    let pos = (lo + hi) * 0.5 + (hi - lo) * 0.5 * corner;

    // Rows of the matrix from em space to clip space.
    let s = camera.screen_to_clip_scale;
    let o = camera.parallax_clip_offset;
    let m0 = vec4f(inst.linear.x * s.x, inst.linear.z * s.x, 0.0, inst.translate.x * s.x - 1.0 + o.x);
    let m1 = vec4f(inst.linear.y * s.y, inst.linear.w * s.y, 0.0, inst.translate.y * s.y + 1.0 + o.y);
    let m3 = vec4f(0.0, 0.0, 0.0, 1.0);
    let p = slug_dilate(pos, corner, m0, m1, m3, 2.0 / abs(s));

    var out: Varying;
    out.position = vec4f(dot(m0.xy, p) + m0.w, dot(m1.xy, p) + m1.w, 0.0, 1.0);
    out.coord = p;
    out.screen = vec2f(
        inst.linear.x * p.x + inst.linear.z * p.y + inst.translate.x,
        inst.linear.y * p.x + inst.linear.w * p.y + inst.translate.y,
    );
    out.color = inst.color;
    out.clip = vec4f(inst.clip_min, inst.clip_max);
    out.layer = inst.layer;
    return out;
}

@fragment
fn fs_main(in: Varying) -> @location(0) vec4f {
    if in.screen.x < in.clip.x || in.screen.y < in.clip.y || in.screen.x > in.clip.z || in.screen.y > in.clip.w {
        discard;
    }
    // At a very high zoom, f32 cannot show a change of coord in one pixel.
    let pixels_per_em = 1.0 / max(fwidth(in.coord), vec2f(1e-9));
    let l = slug_layer_at(in.layer);
    var color = in.color;
    if l.kind == SLUG_SOLID {
        color = l.color;
    }
    return vec4f(color.rgb, color.a * slug_coverage(l, in.coord, pixels_per_em));
}
