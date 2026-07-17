struct Particle {
  pos : vec2f, vel : vec2f,
  age : f32, life : f32,
  size0 : f32, size1 : f32,
  spin : f32, counter : u32,
  wobble_axis : vec2f,
}

struct Counters {
  dead : atomic<u32>,
  alive_cur : atomic<u32>,
  alive_new : atomic<u32>,
  pad : u32,
}

struct DispatchIndirect {
  dx : u32, dy : u32, dz : u32,
}

struct DrawIndirect {
  vertex_count : u32,
  instance_count : atomic<u32>,
  first_vertex : u32, first_instance : u32,
}

struct EmitterUniforms {
  color_start : vec4f, color_end : vec4f,
  to_clip_scale : vec2f, to_clip_offset : vec2f,
  pos : vec2f, prev_pos : vec2f,
  gravity : vec2f, emit_extent : vec2f, wobble_axis : vec2f,
  dt : f32, drag : f32,
  emit_count : u32, emit_base : u32, seed_lo : u32, seed_hi : u32,
  dir_base : f32, spread : f32,
  speed_min : f32, speed_max : f32, life_min : f32, life_max : f32,
  size0_min : f32, size0_max : f32, size1_min : f32, size1_max : f32,
  spin_max : f32, rot_min : f32, rot_max : f32,
  wobble_amp : f32, wobble_freq : f32, fade_in : f32,
  shape : u32, vel_mode : u32, emit_radius : f32, capacity : u32,
}

@group(0) @binding(0) var<storage, read_write> particles : array<Particle>;
@group(0) @binding(1) var<storage, read_write> alive_cur : array<u32>;
@group(0) @binding(2) var<storage, read_write> alive_new : array<u32>;
@group(0) @binding(3) var<storage, read_write> dead_list : array<u32>;
@group(0) @binding(4) var<storage, read_write> counters : Counters;
@group(0) @binding(5) var<storage, read_write> draw_indirect : DrawIndirect;
@group(0) @binding(6) var<storage, read_write> dispatch_indirect : DispatchIndirect;
@group(0) @binding(7) var<uniform> u : EmitterUniforms;

// mirror: particle.pcg3d (hash.go)
fn pcg3d(p : vec3u) -> vec3u {
  var v = p * 1664525u + 1013904223u;
  v.x += v.y * v.z;  v.y += v.z * v.x;  v.z += v.x * v.y;
  v ^= v >> vec3u(16u);
  v.x += v.y * v.z;  v.y += v.z * v.x;  v.z += v.x * v.y;
  return v;
}
// mirror: particle.Float01 — divide by 2^32 so 1.0 is unreachable
fn float01(x : u32) -> f32 { return f32(x) * 2.3283064365386963e-10; }

const TAU = 6.28318530717958647;

@compute @workgroup_size(1)
fn kick() {
  let alive = atomicLoad(&counters.alive_new);
  atomicStore(&counters.alive_cur, alive);
  atomicStore(&counters.alive_new, 0u);
  dispatch_indirect.dx = (alive + u.emit_count + 63u) / 64u;
  dispatch_indirect.dy = 1u;
  dispatch_indirect.dz = 1u;
  draw_indirect.vertex_count = 6u;
  atomicStore(&draw_indirect.instance_count, 0u);
  draw_indirect.first_vertex = 0u;
  draw_indirect.first_instance = 0u;
}

@compute @workgroup_size(64)
fn emit(@builtin(global_invocation_id) gid : vec3u) {
  let i = gid.x;
  if (i >= u.emit_count) { return; }

  // Pop the dead stack; on underflow (pool exhausted) undo and drop the
  // spawn — mirrors the CPU cap-and-drop, never defers.
  let old = atomicSub(&counters.dead, 1u);
  if (old == 0u || old > u.capacity) {
    atomicAdd(&counters.dead, 1u);
    return;
  }
  let slot = dead_list[old - 1u];

  // mirror: particle.Emitter.spawn lane map — h1 angle/speed/life,
  // h2 sizes/spin, h3 shape, h4 rotation/wobble phase (re-derived on draw)
  let h1 = pcg3d(vec3u(u.emit_base + i, u.seed_lo, u.seed_hi));
  let h2 = pcg3d(h1);
  let h3 = pcg3d(h2);

  var offset = vec2f(0.0);
  switch (u.shape) {
    case 1u: { // disc: r = sqrt(u)*R for area uniformity
      let r = u.emit_radius * sqrt(float01(h3.x));
      let a = TAU * float01(h3.y);
      offset = vec2f(cos(a), sin(a)) * r;
    }
    case 2u: { // rect half-extents
      offset = (vec2f(float01(h3.x), float01(h3.y)) * 2.0 - 1.0) * u.emit_extent;
    }
    case 3u: { // ring: lane h3.y, matching the CPU (h3.x unused)
      let a = TAU * float01(h3.y);
      offset = vec2f(cos(a), sin(a)) * u.emit_radius;
    }
    default: {}
  }

  let speed = mix(u.speed_min, u.speed_max, float01(h1.y));
  let base_dir = vec2f(cos(u.dir_base), sin(u.dir_base));
  var dir = base_dir;
  switch (u.vel_mode) {
    case 1u: { // radial; point shapes are rejected at manifest load
      if (dot(offset, offset) > 0.0) {
        dir = normalize(offset);
      }
    }
    case 2u: { // tangential
      var r = base_dir;
      if (dot(offset, offset) > 0.0) {
        r = normalize(offset);
      }
      dir = vec2f(-r.y, r.x);
    }
    default: { // cone (mirror: initialVelocity)
      let a = u.dir_base + (2.0 * float01(h1.x) - 1.0) * u.spread;
      dir = vec2f(cos(a), sin(a));
    }
  }

  // mirror: spawn-origin interpolation t = (i+1)/emitCount
  let t = f32(i + 1u) / f32(u.emit_count);

  var p : Particle;
  p.pos = mix(u.prev_pos, u.pos, t) + offset;
  p.vel = dir * speed;
  p.age = 0.0;
  p.life = mix(u.life_min, u.life_max, float01(h1.z));
  p.size0 = mix(u.size0_min, u.size0_max, float01(h2.x));
  p.size1 = mix(u.size1_min, u.size1_max, float01(h2.y));
  p.spin = (2.0 * float01(h2.z) - 1.0) * u.spin_max;
  p.counter = u.emit_base + i;
  p.wobble_axis = u.wobble_axis;
  particles[slot] = p;

  let idx = atomicAdd(&counters.alive_cur, 1u);
  alive_cur[idx] = slot;
}

@compute @workgroup_size(64)
fn simulate(@builtin(global_invocation_id) gid : vec3u) {
  let i = gid.x;
  if (i >= atomicLoad(&counters.alive_cur)) { return; }
  let slot = alive_cur[i];
  var p = particles[slot];

  p.age += u.dt;
  if (p.age >= p.life) {
    let d = atomicAdd(&counters.dead, 1u);
    dead_list[d] = slot;
    return;
  }
  // mirror: Emitter.Step — vel = (vel + g*dt) * pow(drag, dt)
  p.vel = (p.vel + u.gravity * u.dt) * pow(u.drag, u.dt);
  p.pos += p.vel * u.dt;
  particles[slot] = p;

  let idx = atomicAdd(&counters.alive_new, 1u);
  alive_new[idx] = slot;
  atomicAdd(&draw_indirect.instance_count, 1u);
}
