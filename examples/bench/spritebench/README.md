# Sprite render benchmark

Build and serve the standalone browser benchmark:

```powershell
go run ./cmd/frameforge dev ./examples/bench/spritebench/wasm -a 127.0.0.1:8092
```

It bounces its own slice of sprites (position, velocity, rotation) around a
1920x1080 world and emits one `render.DrawImage` per sprite. In the default
**direct** path the commands are built straight from that slice, bypassing the
ECS so results isolate the tess/upload path: `tess.Build` tessellating draw
commands into vertex streams, the `Bytes`/`TexBytes` conversion into the
WebGPU vertex buffer layout, and the JS-heap upload that follows.

The **ECS** path mirrors the same sprites into `scene.World` entities
(`Transform` + `ImageSprite`, no colliders) and renders through
`World.RenderCommands`, so each frame additionally pays the per-tick
`SetTransform` store writes, the `SyncRenderState` transform clone and the
`RenderCommands` id-sort/command building. The delta between the two paths at
the same sprite count is the ECS render-path overhead, including how the extra
allocation churn interacts with the single-threaded wasm GC (watch P99).
Colliders stay out of both paths on purpose: broad-phase cost would drown the
render-path signal at high counts and has its own benchmark suite.

Controls:

- `UP` doubles the sprite count, `DOWN` halves it (starts at 1,000, clamped to
  [1, 500,000]).
- `T` toggles between one shared texture and four textures interleaved per
  sprite, which forces the textured pipeline to split a run on every draw
  instead of batching the whole frame into one.
- `E` toggles between the direct and ECS render paths at the current count.

The HUD reports sprite count, texture mode, render path, rolling FPS, frame
time, P99 frame time, and the WebGPU render pass GPU time (`GPURenderTime`),
which the renderer samples on every frame whenever the host consumes
`FrameMetrics` — no particles required.

The `cpu ~= frame - gpu draw` line is an estimate of the CPU-side cost of the
render loop: frame time minus the measured GPU draw pass leaves command
building, `tess.Build`, the `Bytes`/`TexBytes` conversions and the
`bytesToJS` heap copies. It is not a measurement of any single stage — use it
to see whether growing the sprite count or switching to four textures shifts
cost onto the CPU path versus the GPU.
