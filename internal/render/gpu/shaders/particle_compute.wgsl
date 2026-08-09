// Particle state is split by mutability so the simulate pass touches only the
// 24 hot bytes: everything else is written once at spawn and read by render.
struct ParticleHot {
  pos : vec2f, vel : vec2f,
  age : f32, life : f32,
}

struct ParticleCold {
  size0 : f32, size1 : f32,
  spin : f32, rot0 : f32,
  wobble_axis : vec2f,
  phase : f32,
}

// dead is the free-slot stack cursor and persists across frames; alive is the
// survivor-list write cursor, reset by the host before each simulated frame.
struct Counters {
  dead : atomic<u32>,
  alive : atomic<u32>,
  pad0 : u32,
  pad1 : u32,
}

struct DrawIndirect {
  vertex_count : u32,
  instance_count : atomic<u32>,
  first_vertex : u32, first_instance : u32,
}

struct EmitterUniforms {
  color_start : vec4f, color_end : vec4f,
  world_to_clip_x : vec2f, world_to_clip_y : vec2f,
  to_clip_offset : vec2f,
  world_to_screen_scale : vec2f,
  screen_to_clip_scale : vec2f,
  pos : vec2f, prev_pos : vec2f,
  gravity : vec2f, emit_extent : vec2f, emit_scale : vec2f, wobble_axis : vec2f,
  dt : f32, drag : f32,
  emit_count : u32, emit_base : u32, seed_lo : u32, seed_hi : u32,
  dir_base : f32, spread : f32,
  speed_min : f32, speed_max : f32, life_min : f32, life_max : f32,
  size0_min : f32, size0_max : f32, size1_min : f32, size1_max : f32,
  spin_max : f32, rot_min : f32, rot_max : f32,
  wobble_amp : f32, wobble_freq : f32, fade_in : f32, flip_x_freq : f32,
  shape : u32, vel_mode : u32, emit_radius : f32, capacity : u32,
}

@group(0) @binding(0) var<storage, read_write> hot : array<ParticleHot>;
@group(0) @binding(1) var<storage, read_write> alive_list : array<u32>;
@group(0) @binding(2) var<storage, read_write> dead_list : array<u32>;
@group(0) @binding(3) var<storage, read_write> counters : Counters;
@group(0) @binding(4) var<storage, read_write> draw_indirect : DrawIndirect;
@group(0) @binding(5) var<uniform> u : EmitterUniforms;
@group(0) @binding(6) var<storage, read_write> cold : array<ParticleCold>;

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
  // h2 sizes/spin, h3 shape, h4 rotation/wobble+flip phase. h4 lanes are
  // stored on the particle so the render vertex stage never re-hashes.
  let h1 = pcg3d(vec3u(u.emit_base + i, u.seed_lo, u.seed_hi));
  let h2 = pcg3d(h1);
  let h3 = pcg3d(h2);
  let h4 = pcg3d(h3);

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
  offset *= u.emit_scale;

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

  var p : ParticleHot;
  p.pos = mix(u.prev_pos, u.pos, t) + offset;
  p.vel = dir * speed;
  p.age = 0.0;
  // life > 0 marks the slot live for the sequential simulate scan; a decl
  // with a zero lifetime still dies on its first step, matching the CPU.
  p.life = max(mix(u.life_min, u.life_max, float01(h1.z)), 1e-6);
  hot[slot] = p;

  var c : ParticleCold;
  c.size0 = mix(u.size0_min, u.size0_max, float01(h2.x));
  c.size1 = mix(u.size1_min, u.size1_max, float01(h2.y));
  c.spin = (2.0 * float01(h2.z) - 1.0) * u.spin_max;
  c.rot0 = mix(u.rot_min, u.rot_max, float01(h4.x));
  c.wobble_axis = u.wobble_axis;
  c.phase = float01(h4.y);
  cold[slot] = c;
}

// The simulate pass scans pool slots directly instead of chasing a compacted
// alive list: thread i owns hot[i], so reads and writes are fully coalesced —
// the dominant cost of the previous indirection was its scattered access, not
// arithmetic. Dead slots (life == 0, the zero-initialized state) cost one
// sequential 24-byte read each. The survivor list for the render stage is
// rebuilt every simulated frame with one workgroup-aggregated range per list:
// threads take a lane from a workgroup-local counter, thread 0 reserves the
// global range, and survivors/dead scatter into it. Order is
// workgroup-relative, but the per-thread atomics it replaced never had a
// deterministic order either.
var<workgroup> wg_alive_count : atomic<u32>;
var<workgroup> wg_dead_count : atomic<u32>;
var<workgroup> wg_alive_base : u32;
var<workgroup> wg_dead_base : u32;

@compute @workgroup_size(64)
fn simulate(@builtin(global_invocation_id) gid : vec3u,
            @builtin(local_invocation_index) lid : u32) {
  if (lid == 0u) {
    atomicStore(&wg_alive_count, 0u);
    atomicStore(&wg_dead_count, 0u);
  }
  workgroupBarrier();

  let slot = gid.x;
  var lane = 0u;
  var alive = false;
  var died = false;
  if (slot < u.capacity) {
    var p = hot[slot];
    if (p.life > 0.0) {
      p.age += u.dt;
      if (p.age >= p.life) {
        // Exactly-once death: life = 0 frees the slot for the scan while the
        // dead-stack push below returns it to the emit pass.
        p.life = 0.0;
        hot[slot] = p;
        died = true;
        lane = atomicAdd(&wg_dead_count, 1u);
      } else {
        // mirror: Emitter.Step — vel = (vel + g*dt) * pow(drag, dt)
        p.vel = (p.vel + u.gravity * u.dt) * pow(u.drag, u.dt);
        p.pos += p.vel * u.dt;
        hot[slot] = p;
        alive = true;
        lane = atomicAdd(&wg_alive_count, 1u);
      }
    }
  }

  workgroupBarrier();
  if (lid == 0u) {
    let alive_n = atomicLoad(&wg_alive_count);
    if (alive_n > 0u) {
      wg_alive_base = atomicAdd(&counters.alive, alive_n);
      atomicAdd(&draw_indirect.instance_count, alive_n);
    }
    let dead_n = atomicLoad(&wg_dead_count);
    if (dead_n > 0u) {
      wg_dead_base = atomicAdd(&counters.dead, dead_n);
    }
  }
  workgroupBarrier();

  if (alive) {
    alive_list[wg_alive_base + lane] = slot;
  } else if (died) {
    dead_list[wg_dead_base + lane] = slot;
  }
}
