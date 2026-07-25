# Scene simulation benchmark

Build and serve the standalone browser benchmark:

```powershell
go run ./cmd/Arasy dev ./examples/bench/scenebench/wasm -a 127.0.0.1:8095
```

It spawns N `scene.World` entities with `Transform` + `Velocity` +
`CircleSprite` (plus `CircleCollider` when colliders are on) and lets the
engine's own `World.Step` pipeline move and bounce them every tick
(acceleration -> integrate -> bounce -> grid broad-phase -> contact
resolution -> damping). Unlike `spritebench`, which deliberately keeps
colliders out of both its render paths, this benchmark exists specifically to
measure the collision broad-phase cost that spritebench excludes: positions
are never mirrored from a slice, so the tick cost measured is the real
`scene.Step` cost.

Controls:

- `UP` doubles the entity count, `DOWN` halves it (starts at 1,000, clamped
  to [1, 100,000]).
- `E` toggles `CircleCollider` on or off for every entity at the current
  count. Toggling at the same N isolates the grid broad-phase and contact
  resolution cost from the rest of the tick.

The HUD reports entity count, collider mode, rolling FPS, frame time, P99
frame time, the tick's broad-phase pair and contact counts (from
`scene.StepStats` via the optional `PostStepper` interface), the WebGPU
render pass GPU time, and the same CPU tick/render and
tessellation/upload/encode breakdown reported by the other benchmarks. The
tick vs. render split in the CPU line is the key readout: with colliders on,
growing the entity count should shift cost onto the tick side; with colliders
off, the same growth should show up almost entirely in scene traversal
instead.

Entity count changes despawn and respawn the whole group through
`World.DespawnBatch`, which compacts each component store once (O(n) per
store) instead of paying the O(k*n) cost of removing entities one at a time.
