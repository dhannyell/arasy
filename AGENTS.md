# AGENTS.md

Instructions for IA agents working in this repository.

## Commands

```powershell
# Tests
go test ./...                          # all tests
go test ./scene -run TestStep          # single test
go test ./scene -run="^$" -bench=. -benchmem  # benchmarks
golangci-lint run ./...                # mandatory before every handoff/commit

# Benchmarks (suite registry: cmd/benchsave/suites.go)
go run ./cmd/benchsave -list-suites
go run ./cmd/benchsave -suite <name>                   # run one suite
go run ./cmd/benchsave -suite all                      # run all suites
go run ./cmd/benchsave -suite <name> -compare baseline # vs explicit baseline
go run ./cmd/benchsave -suite <name> -compare auto     # vs previous report
go run ./cmd/benchsave -suite <name> -update-baseline  # set new baseline
go run ./cmd/benchsave -diff <old> <new>               # diff reports
go run ./cmd/benchsave -compare-all                    # diff all families

# Demos
go run ./cmd/demos/football/raster               # offline PNG/GIF
go run ./cmd/demos/assets/raster                 # offline asset pipeline PNG
go run ./cmd/demos/assets/canvas                 # browser asset pipeline :8080
go run ./cmd/demos/football/canvas               # browser WS :8080
go run ./cmd/demos/breakout/canvas                # browser WS :8080
# WASM build then serve:
go run ./cmd/wasmserve -dir output/wasm/breakout -addr 127.0.0.1:8090

# Rhythm: offline smoke render, then dev server (builds + rebuilds on browser reload).
go run ./examples/rhythm/raster
go run ./cmd/Arasy dev ./examples/rhythm/wasm -a 127.0.0.1:8090
go run ./cmd/Arasy build ./examples/breakout/wasm -o dist/breakout

# Golden files (regenerate after intentional visual changes)
go test ./platform/raster -run TestGolden -update
npm run update:wasm-golden
go test ./examples/breakout ./examples/2048 ./examples/rhythm -run TestGoldenReplay -update
```

Golden PRs must explain the visual change; unexplained diffs are regressions.
Golden replays (`internal/replaytest`, committed `.arrp` + `.state.txt`
fixtures under each demo's `testdata/`) follow the same rule for simulation
changes.

## Architecture

```
Driver.Step/AdvanceTime -> Game.Update -> World.Step/PostStep -> render commands -> platform renderer
```

### Layers

```
foundation:     math2d
vocabulary:     render, input, ui, audio, particle
simulation:     scene
orchestration:  runtime
hosts:          platform/raster, platform/wasm, platform/native
apps:           cmd/
```

**Dependency direction** (never reverse):
```
cmd/demos -> platform/* -> runtime -> {scene, input, ui, render, audio, asset} -> math2d
asset -> {render, audio, particle} -> math2d
platform/* -> asset/pixel -> asset
```

### Rules
- `math2d`, `render`, `input`, `ui`, `scene`, `runtime` never import `platform/*`.
- `runtime` is the sole behavioral API for platforms/games. Platforms call `runtime.Driver` methods only; never `scene.World` directly.
- `runtime` is backend-free: no JSON, HTTP, images, browser concepts. Manifest parse lives in `asset`; `runtime` only transports the resolved `asset.Manifest`.
- Games implement `runtime.Game`; World arrives via `UpdateContext`.

### Package Responsibilities

| Package | Role                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                     |
|---|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `math2d/` | Pure 2D math (`Vec2`, `Rect`). Zero engine deps.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                         |
| `render/` | Backend-agnostic commands, `Color`, `MeasureText`, `Camera`, `CommandSink` and stable `Each` dispatch; `DrawText.Font` selects a per-run `FontID`, `DrawImageQuad` maps a texture onto a convex projective quad, `DrawEllipse` renders axis-aligned ellipses with optional projection, and `DrawParticles` drives manifest-declared emitters. `BaseCommand.Transform` carries an affine matrix per command (zero value = identity; decorative only — hit-testing deferred). Every backend transforms the full geometry per vertex (shapes) or inverse-samples the transformed quad (`DrawImage`/`DrawImageQuad`), so rotation/shear/non-uniform scale agree across raster, Canvas2D and WebGPU; `DrawParticles` is the exception (its emitter simulation ignores `Transform`). `DrawSpriteBatch`/`DrawTextBatch` draw a retained `SpriteBatchID`/`TextBatchID` published through `runtime.BatchPort` (see Retained Render Batches below). Raster is the geometry/color reference; Canvas2D hosts approximate quads with an affine grid.                  |
| `internal/renderbatch/` | Backend-mirrorable helpers for retained batches: defensive copies on intake, world bounds, flip-book/bounce/pulse animation evaluation, and conservative culling inflation/view intersection. Raster, Canvas2D and the WGSL shaders all evaluate animation through this package's rules.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                 |
| `internal/render/gpu/tess/` | Shared backend-agnostic GPU core that tessellates render commands into ordered streams per WebGPU pipeline. It is consumed by the WASM WebGPU host and, in Phase C, by the native host. |
| `internal/render/gpu/fontatlas/` | Shared manifest-backed font atlas provider. It creates deterministic glyph atlases and metrics consumed by both the WASM WebGPU host and the native renderer. |
| `internal/render/gpu/shaders/` | Shared embedded WGSL sources consumed by the WASM WebGPU host and, in Phase C, by the native host. |
| `asset/` | Data-driven texture/sprite/font/audio declarations from layered `fs.FS` manifests (last layer wins). `render.TextureID` and `render.FontID` are stable per-run, resolved by name, and never persisted. `asset/pixel` owns shared image decode; `asset/typeface` owns deterministic SFNT/OpenType metrics and parsing; `runtime` carries only the resolved `asset.Manifest`.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                              |
| `audio/` | Backend-agnostic audio vocabulary: clip IDs, one-shot cues, continuous tracks, and rewindable track state. Hosts decode/play; runtime only transports intent.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                            |
| `particle/` | Render-only effect vocabulary plus the CPU semantic reference: fixed emitter pools, deterministic spawn hashing, active-slot bitsets, emission accounting, and adaptive fallback budgets. WebGPU mirrors the same spawn semantics in compute shaders.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                    |
| `scene/` | ECS world + simulation tick. `EntityID` (uint32). `ImageSprite` is the texture-backed visual component and is anchored to `Transform` like `CircleSprite`. Step pipeline: locomotion → acceleration → ballistics (drag, Magnus, gravity) → integrate → bounce → grid broad-phase → contact resolution → damping. Supports 2.5D physics via `Height` (scalar altitude), `Ballistics` (projectile flight with quadratic drag, Magnus curve, ground bounce/roll, spin decay, and opt-in `Substeps` continuous collision against static walls), `Spin` (vertical-axis angular velocity), `Locomotion` + `MoveIntent` (steering with asymmetric acceleration and speed-dependent turn radius). Render-only depth sorting by `(Layer, Y, EntityID)` with `Height` interpolation and `render.Projection` for 2.5D visual lift/scale plus a ground-shadow `DrawEllipse` emitted with each lifted body. `World.Checksum()` for determinism covers all sim-state components; `renderHeights` and `projection` are presentation caches excluded from checksum.                                                                                                                                        |
| `input/` | `Source`/`Action`/`Binding`/`Mapper`/`Buffer` + pointer types. `Mapper.EndFrame()` called by driver at tick end.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                         |
| `ui/` | Immediate-mode: Button, Panel, Label, etc. Three coord spaces. `ui.State` owned by driver (captured in rewind).                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                          |
| `runtime/` | Orchestration hub. Owns `Game` contract, `Driver` (fixed timestep, dev controls, rewind), `FrameTimer`, `FrameStatsTracker`, `RunTick`, `StateSlot[T]`, and `ConsumeAudio`. `FrameStatsTracker` is the shared frame-pacing surface consumed by WASM and native hosts; native GPU timing is an opt-in coarse `Poll` probe because the binding does not expose per-pass timestamp writes. `BatchPort` (shared by `UpdateContext`/`RenderContext`) publishes retained sprite/text batch content; `FrameRenderer.UpdateResources(FrameUpdates)` drains textures and batches before `Render`. Defines *what a tick is* -- platforms must not duplicate.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                        |
| `loop/` | Fixed timestep step planner. Runtime clock uses it; platforms never choose `dt`.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                         |
| `platform/raster/` | Software renderer -> `image.RGBA`. Offline host (PNG/GIF via `Driver`). Semantic reference for retained-batch culling and painter order.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                 |
| `platform/wasm/` | Browser host (`GOOS=js`): `requestAnimationFrame`, WebGPU (default) + Canvas2D fallback. WebGPU uploads manifest textures and per-`(FontID, pixelSize)` `r8unorm` glyph atlases (`FontID(0)` remains the built-in bitmap font), generates texture mip chains at boot, uses linear anisotropic image sampling, and simulates particles in compute with indirect draws. Retained sprite/text batches reuse the same storage-buffer vertex-pulling pipelines as streaming content (no `stepMode: "instance"`); a batch uploads only on replace/delete, camera/projection change, or (for text) atlas revision. Canvas2D approximates image quads and particles on the CPU with an adaptive budget, caps retained batches at 2000 sprites / 500 labels (dev-only; WebGPU/raster are the reference above the cap), and paints external fonts through the browser FontFace API (monospace until the FontFace loads or when it is unavailable). Particles are render-only; raster remains their semantic reference. Dev panel (backtick key) for debug controls. |
| `platform/native/` | Windows/amd64 native host and streaming renderer over wgpu-native and GLFW. `native.Run`/`RunDriver` accept `Options` for window title, size, fullscreen, VSync, icon, logging, and the native `FrameMetrics` callback. The host reports CPU tick/render, input latency and Go memory; the renderer reports CPU phases and particle buffer bytes. GPU timing is unavailable because the binding's completion callback is unsafe, while per-pass timestamps are unavailable in the binding and retained batches are intentionally unsupported. The host locks its OS thread; GLFW callbacks only queue input, and the loop applies it through `Driver.RunTick` with a framebuffer-pixel viewport before rendering. Alt+Enter toggles borderless fullscreen. The renderer consumes the shared GPU tessellation and WGSL core, renders solid/textured/sprite/text streams and direct particle compute/indirect draws with MSAA and manifest resources; retained batches remain intentionally skipped with one-time diagnostics. Its process-global oto sink decodes manifest WAV/MP3/OGG clips, reconciles cues and tracks with loop/gain/pan/fade, consumes output after `RunTick` and before rendering, and suspends/resumes on focus changes. |
| `cmd/` | Demos and tools. Game code lives here, not in engine packages.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                           |

### Runtime Tick Semantics

- Platforms drive time via `Driver.AdvanceTime(elapsed)`. Driver owns `dt` = `1/SimulationHz`.
- Interactive hosts use `runtime.FrameTimer` for first-frame/activation timing and `Driver.RunTick` for the canonical viewport → controls → input → advance → audio order.
- `AdvanceTime`: consume dev controls -> apply reset/rewind -> convert wall time x speed -> fixed steps with catch-up clamp -> run each step.
- `Driver.Step()`: exactly one fixed tick, no wall clock (for tests/replay).
- Per step: capture rewind snapshot -> snapshot input -> `Game.Update` -> auto `World.Step` (locomotion → acceleration → ballistics → integrate → bounce → collision → damping; unless `ctx.HoldSimulation()`) -> optional `PostStep` -> advance tick -> clear pointer frame -> `mapper.EndFrame()` -> prune input buffer.
- Hosts drain audio cues/tracks with `Driver.ConsumeAudio` from the simulation loop after `AdvanceTime`/`Step`, never from render.
- Input applied *before* `AdvanceTime`/`Step` via `ApplyInput`, stamped with current tick.
- **Do not reorder.** Determinism and rewind depend on this pipeline.
- `StateSlot[T]` snapshots by value: T must be a pure value type (no slices/maps/pointers), or implement `Clone() T` (`runtime.Cloneable`) to deep-copy reference fields — the hook runs on capture and on restore.

### ECS Pattern -- Adding a Component

1. Define struct in `scene/components.go`
2. Add `store[ComponentType]` field to `World`
3. Add `Set`/getter/`Remove` methods delegating to store, following the
   `SetX(EntityID, T)` + `X(EntityID) (T, bool)` convention exactly — the
   structural guards discover components by reflecting over this convention
4. Clone in `World.Clone()` (structural guard: TestCloneCoversAllComponentStores)
5. Remove in `World.Despawn()` (structural guard: TestDespawnCoversAllComponentStores)
6. Include in `World.Checksum()` + bump `ChecksumComponentStores` (structural guard test fails if you forget)

### Simulation Determinism
- No `map` iteration in sim path. Component stores are ordered by EntityID. Grid broad-phase (`collision.go`) is reference pattern for deterministic aux structures.
- Use `ctx.Rand` (not global RNG) in `Update`. Render never draws random values (frames != ticks).
- `World.Checksum()` + driver debug log verify sync.

### Test Structure
- Follow `docs/testing.md`. Name files by behavior, not one broad `*_test.go` per package.
- `package foo_test` by default; `package foo` only when testing internals.
- Helpers: call `t.Helper()`, move to shared file when used across several test files.
- Benchmarks go in dedicated `*_benchmark_test.go` files.
- `internal/simfuzz` provides twelve fuzz targets for simulation determinism (see `docs/testing.md`).

## Render Interpolation

The engine provides built-in render interpolation for official platforms (WASM,
offline raster, future native).

- runtime.Config.RenderInterpolation (default false) enables the feature.
- runtime.RenderContext.Alpha exposes the [0, 1) fraction between ticks.
- scene.World.RenderCommandsInterpolated(alpha) returns interpolated commands.
- scene.World.SyncRenderState() captures the previous tick's transforms.
- Interpolation is a presentation cache: it never enters Checksum(), replay,
  rewind snapshots, or persisted recordings.
- SetTransform immediately syncs the render state for that entity to prevent
  visual smear on teleport/reset.
- Only transform properties are interpolated (Position, Rotation, Scale via
  Vec2.Lerp and math2d.Lerp). Debug overlays (colliders, grid, velocity)
  always show the current simulation state.
- Games opt in by calling ctx.World.RenderCommandsInterpolated(ctx.Alpha)
  in their Render method.

## Retained Render Batches

`render.DrawSpriteBatch`/`render.DrawTextBatch` draw content published once
through `runtime.BatchPort` (`UpdateSprites`/`DeleteSprites`/`UpdateText`/
`DeleteText`, shared by `Game.Init`, `Game.Update` and `Game.Render`) instead
of every frame like `DrawImage`/`DrawText` streaming.

- Render-only: batch content never enters `Checksum()`, snapshots, replays or
  recordings; rewind neither drops nor undoes a publication.
- A batch command is a painter-order boundary and owns exactly one layer and
  one clip; a replace is a full-content swap (no partial/dirty-range update
  in v1) and an empty replace keeps an empty batch -- only `Delete*` removes
  it. A command referencing a missing/deleted batch is skipped with a
  one-time warning.
- Culling is conservative and whole-batch: world bounds are computed on
  update and inflated by animation at draw time; there is no per-instance
  culling, compute, or reordering.
- GPU animation (`SpriteAnimation` flip-book/phase/bounce, `SpritePulse`,
  `SpriteBatchAnimation`) is visual-only, closed vocabulary mirrored across
  raster/Canvas2D/WGSL by `internal/renderbatch` -- no arbitrary shaders, no
  gameplay state.
- WebGPU reuses the streaming storage-buffer/vertex-pulling pipelines; stable
  content and projection upload zero bytes per frame. Raster is the
  reference implementation; Canvas2D is dev-only and caps batch size (see
  `platform/wasm/` above).
- Retained sprites are for stable or rarely-changing content (crowd,
  scenery, HUD/tables), not a replacement for fully dynamic ECS sprites,
  which stay on the streaming path.

## Design Constraints
- Engine packages stay independent of app-specific rules.
- Prefer small explicit APIs; add complexity under project pressure.
- Render commands stay backend-agnostic. Measure before optimizing.
- Stdlib preferred; engine packages keep third-party dependencies limited to `golang.org/x/image` where already used. CLI tooling under `internal/cli` may use `spf13/cobra`.

## Float Discipline

Sim path (`scene/`, `runtime/`, `math2d/`): only `+ - * /` and `math.Sqrt`. Transcendentals forbidden by `forbidigo` (see `.golangci.yml`).

**FMA:** arm64 fuses `a*b + c`. Mitigation: `float64(a*b) + c`.
```go
func (v Vec2) Dot(o Vec2) float64 {
    return float64(v.X*o.X) + float64(v.Y*o.Y)
}
```
Review sum-of-product expressions for explicit `float64()` on the product operand.
## CI Gates

| Job | Checks |
|---|---|
| `quality` | actionlint, golangci-lint (forbidigo), govulncheck |
| `test` | `gofmt`, `go vet`, `go test ./...` (x86-64) |
| `no-default-font` | `go test -tags arr_no_default_font ./...` + js/wasm build+vet with the tag (the embedded default font compiled out) |
| `race` | `go test -race ./...` |
| `windows` | `go test ./...` (Windows compat) |
| `wasm` | `GOOS=js go build ./...` + `go vet ./...` + sim-path tests via `go_js_wasm_exec` (incl. `examples/...` golden replays) + Canvas2D visual golden; WebGPU golden remains a local/browser gate |
| `arm64` | `go test ./...` (native arm64) |

`wasm` + `arm64` = cross-arch determinism net. Golden checksum (`scene/testdata/checksum_golden.json`) must match x86-64, arm64, and wasm. Regenerate in a dedicated commit on x86-64 when format changes. Golden replays (`examples/*/testdata/*.arrp`) run on all four Go architectures (x86-64, windows, arm64, wasm) via `go test ./examples/...`, extending the same net to the full driver+input+UI+audio+game pipeline.

## Embedded Default Font

The engine embeds Go Mono (BSD-3, via `golang.org/x/image/font/gofont/gomono`)
as the default font for `FontID(0)`. Text layout for `font 0` still uses
`render.MeasureText` (monospace 0.6 advance, deterministic). Builds with

    Arasy build --no-default-font

exclude the ~140 KB TTF, falling back to the built-in 7×13 bitmap font.

Raster goldens whose frames draw text with `FontID(0)` exist in both variants:
`platform/raster/testdata/golden/` for the embedded font and
`platform/raster/testdata/golden/bitmap/` for the opt-out. Regenerate the
second set with

    go test -tags arr_no_default_font ./platform/raster -run TestGolden -update

Both sets stay pixel-exact; the `no-default-font` CI job guards the opt-out.

## Reference Docs
- `README.md` -- capabilities, quick start, demo instructions
- `docs/runtime.md` -- runtime guide (tick semantics, Driver API, rewind)
- `docs/audio.md` -- audio guide (cues, tracks, host contract, rewind/scrub policy)
- `docs/ui.md` -- UI guide (themes, coord spaces, interaction)
- `docs/scrubbing.md` -- bidirectional time scrubbing (replay, timeline forking, checksum)
- `docs/testing.md` -- test organization conventions
