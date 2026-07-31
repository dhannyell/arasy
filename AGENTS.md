# AGENTS.md

Instructions for AI agents that work in this repository.

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
# Native demos open the dev panel only in dev builds:
go run -tags arr_dev ./examples/breakout/native
go run ./cmd/demos/assets/raster                 # offline asset pipeline PNG
go run ./cmd/demos/assets/canvas                 # browser asset pipeline :8080
go run ./cmd/demos/football/canvas               # browser WS :8080
go run ./cmd/demos/breakout/canvas               # browser WS :8080
# WASM build then serve:
go run ./cmd/wasmserve -dir output/wasm/breakout -addr 127.0.0.1:8090

# Rhythm: offline smoke render, then dev server (rebuilds on browser reload)
go run ./examples/rhythm/raster
go run ./cmd/Arasy dev ./examples/rhythm/wasm -a 127.0.0.1:8090
go run ./cmd/Arasy build ./examples/breakout/wasm -o dist/breakout

# Watch a recorded fuzz case (corpus + paired .arrp, e.g. internal/simfuzz/testdata/{fuzz,replays}):
# offline GIF by default; --live opens a native window (Windows/amd64 only);
# --browser serves internal/simfuzz/wasm with the dev panel (Backquote/~).
go run ./cmd/Arasy watch <corpus> <replay>.arrp [--live | --browser]

# Golden files (regenerate only after an intentional visual change)
go test ./platform/raster -run TestGolden -update
npm run update:wasm-golden
go test ./examples/breakout ./examples/2048 ./examples/rhythm -run TestGoldenReplay -update
```

Golden PRs must explain the visual change; an unexplained diff is a
regression. Golden replays (`internal/replaytest`, committed `.arrp` +
`.state.txt` under each demo's `testdata/`) follow the same rule for
simulation changes.

## Architecture

```
Driver.Step/AdvanceTime -> Game.Update -> World.Step/PostStep -> render commands -> platform renderer
```

```
foundation:     math2d
vocabulary:     render, input, ui, audio, particle
simulation:     scene
orchestration:  runtime
hosts:          platform/raster, platform/wasm, platform/native
apps:           cmd/
```

**Dependency direction** (do not reverse):
```
cmd/demos -> platform/* -> runtime -> {scene, input, ui, render, audio, asset} -> math2d
asset -> {render, audio, particle} -> math2d
platform/* -> asset/pixel -> asset
```

### Rules
- `math2d`, `render`, `input`, `ui`, `scene`, `runtime` must not import `platform/*`.
- `runtime` is the only behavioral API for platforms and games. Platforms call only `runtime.Driver` methods, never `scene.World` directly.
- `runtime` is backend-free: no JSON, HTTP, images, or browser concepts. `asset` parses manifests; `runtime` only carries the resolved `asset.Manifest`.
- Games implement `runtime.Game`. The World arrives through `UpdateContext`.

### Package Responsibilities

| Package | Role |
|---|---|
| `math2d/` | Pure 2D math: `Vec2`, `Rect`. No engine dependencies. |
| `render/` | Backend-agnostic commands (`Color`, `MeasureText`, `Camera`, `CommandSink`, stable `Each`). `DrawImageQuad` maps a texture onto a convex quad; `DrawEllipse` draws axis-aligned ellipses; `DrawParticles` drives manifest emitters. `BaseCommand.Transform` is a per-command affine matrix (identity default, decorative only, no hit-testing). All backends transform shape geometry per vertex, and inverse-sample the quad for `DrawImage`/`DrawImageQuad`, so transforms agree across raster/Canvas2D/WebGPU — except `DrawParticles`, which ignores `Transform`. `DrawSpriteBatch`/`DrawTextBatch` draw retained batches published through `runtime.BatchPort` (see Retained Render Batches). `MSAAMode` is the backend-agnostic anti-aliasing level both GPU hosts take in their `Options`; hosts that cannot multi-sample (raster, Canvas2D) ignore it. Raster is the reference for geometry and color. |
| `internal/devpanel/` | Portable dev tooling overlay (`ui` widgets → render commands): tabs for sim controls, timeline scrubbing, recording/watch, perf (frame-time graph, p99, heap, input latency), inspection (game state via `StateSlot` plus world entities/components by reflection, with click-to-pick in world space and live scalar/bool editing through the world's own setters — edits are not journaled, so an edited session no longer replays exactly), and debug layers. Backend-free — mounted identically by `platform/wasm` and `platform/native`, and tested headlessly. Only compiled under the `arr_dev` build tag (`arasy dev` adds it, `arasy build --devtools` opts in); hosts reference it through a tag-gated seam, so release binaries do not contain the panel. Owns a `ui.State` separate from the game's, so panel state never enters snapshots or checksums. Input follows the capture-flag pattern: every event reaches the widgets, then the pointer is kept while the panel is hovered **or** a widget is active and the keyboard only while one holds focus; captured events never reach `ApplyInput` or a recording. Commands are emitted above every game layer, and a closed panel emits none. `RecordingPort` is the only per-host seam (Blob/picker in the browser, `recordings/` on the desktop). |
| `internal/renderbatch/` | Backend-mirrored helpers for retained batches: defensive copies and world bounds on intake, flip-book/bounce/pulse animation evaluation, conservative culling inflation/view intersection. Raster, Canvas2D, and WGSL shaders evaluate animation through these rules. |
| `anim/` | Stateless, presentation-only timelines (`Timeline`, `Track`). The caller owns the clock and evaluates a `Timeline` at its own elapsed time each call — no `Advance(dt)`, no internal mutation. `ui.Sequence` aliases `Timeline`. See `docs/animation.md`. |
| `render/vector/` | Immutable vector path geometry (`Path`, `Builder`): cubic/quadratic Bézier only (no arc verb), adaptive-tolerance flattening into `render.DrawTriangles`, partial stroke via `TrimStart`/`TrimEnd`, motion sampling via `PointAt`. Fallible operations return a sentinel error, never panic. See `docs/vector-graphics.md`. |
| `internal/svggen/` | Tool-time compiler (`arasy svg` CLI): a secure animated SVG/CSS subset to deterministic `render.Builder`/`render/vector` Go code, no runtime XML/CSS dependency. Always rejects unsafe content (scripts, external URLs); rejects out-of-subset attributes under `--strict` (CLI default). See `docs/vector-graphics.md`. |
| `internal/render/gpu/tess/` | Shared, backend-agnostic GPU core: tessellates render commands into ordered per-pipeline streams. Used by the WASM WebGPU host now; the native host in Phase C. |
| `internal/render/gpu/fontatlas/` | Shared, manifest-backed font atlas provider: deterministic glyph atlases and metrics for the WASM WebGPU host and the native renderer. |
| `internal/render/gpu/shaders/` | Shared, embedded WGSL sources. Used by the WASM WebGPU host now, the native host in Phase C. |
| `asset/` | Data-driven texture/sprite/font/audio declarations from layered `fs.FS` manifests (last layer wins). `render.TextureID`/`render.FontID` are stable per-run, resolved by name, never persisted. `asset/pixel` owns image decode; `asset/typeface` owns SFNT/OpenType metrics and parsing. `runtime` carries only the resolved `asset.Manifest`. |
| `audio/` | Backend-agnostic audio vocabulary: clip IDs, one-shot cues, continuous tracks, rewindable track state. Hosts decode/play; `runtime` only transports intent. |
| `particle/` | Render-only effect vocabulary plus the CPU semantic reference: fixed emitter pools, deterministic spawn hashing, active-slot bitsets, emission accounting, adaptive fallback budgets. WebGPU mirrors the same spawn rules in compute shaders. |
| `scene/` | ECS world + simulation tick (`EntityID` = `uint32`). `ImageSprite` is the texture-backed visual component, anchored to `Transform` like `CircleSprite`. Step pipeline: locomotion → acceleration → ballistics (drag, Magnus, gravity) → integrate → bounce → grid broad-phase → contact resolution → damping. 2.5D physics: `Height` (altitude), `Ballistics` (drag/Magnus/bounce/roll/spin decay, opt-in `Substeps` for continuous wall collision), `Spin` (angular velocity), `Locomotion`+`MoveIntent` (steering). Render-only depth sort by `(Layer, Y, EntityID)` with `Height` interpolation; `render.Projection` adds 2.5D lift/scale and a ground-shadow `DrawEllipse` (`ShadowSkewPerZ` leans it); `SpriteTransform` is a render-only per-entity affine (squash & stretch). `World.Checksum()` covers all sim-state components; `renderHeights`, `spriteTransforms`, `projection` are presentation caches, excluded from checksum. |
| `input/` | `Source`/`Action`/`Binding`/`Mapper`/`Buffer` + pointer and keyboard types. Keys are buttons with a physical code. A typed character is an `EventText` event; `Value` holds its Unicode codepoint. The recording format writes text events with no new bytes. `KeyFrame` is the keyboard equivalent of `PointerFrame`: the text and the editing-key edges of one tick. `Mapper` ignores text events. Driver calls `Mapper.EndFrame()` at tick end. |
| `ui/` | Immediate-mode widgets (Button, Panel, Label, TextInput, Checkbox, Dropdown, Tabs, Table, List, ...), three coord spaces. A widget with a value is caller-owned: the game gives its value and stores the returned value. The value rewinds with the simulation. `ui.State` keeps only widget-internal data: caret, key-repeat clock, open popup, scroll. The Driver owns `ui.State` and captures it in rewind. Popups draw in the reserved `LayerOverlay` block. |
| `runtime/` | Orchestration hub: owns `Game`, `Driver` (fixed timestep, dev controls, rewind), `FrameTimer`, `FrameStatsTracker`, `RunTick`, `StateSlot[T]`, `ConsumeAudio`. `FrameStatsTracker` is the shared frame-pacing surface for WASM and native hosts; native GPU timing is an opt-in coarse `Poll` probe (the binding has no per-pass timestamps). `BatchPort` (shared by `UpdateContext`/`RenderContext`) publishes retained batch content; `FrameRenderer.UpdateResources` drains textures/batches before `Render`. Defines what a tick is — platforms must not duplicate it. |
| `loop/` | Fixed-timestep step planner. The runtime clock uses it; platforms never choose `dt`. |
| `platform/raster/` | Software renderer to `image.RGBA`. Offline PNG/GIF host via `Driver`. Semantic reference for retained-batch culling and painter order. |
| `platform/wasm/` | Browser host (`GOOS=js`): `requestAnimationFrame`, WebGPU by default with a Canvas2D fallback. WebGPU uploads manifest textures and per-`(FontID, pixelSize)` glyph atlases (`FontID(0)` stays the built-in bitmap font), generates mip chains at boot, uses anisotropic sampling, and simulates particles in compute with indirect draws; retained batches reuse the streaming storage-buffer pipelines and re-upload only on replace/delete/camera change/atlas revision. Canvas2D (dev-only) approximates quads and particles on the CPU, caps retained batches at 2000 sprites/500 labels, and paints external fonts via the browser FontFace API. Particles are render-only; raster is their reference. `Options.MSAA` takes the same backend-agnostic `render.MSAAMode` as the native host (zero value 4x, `render.MSAAOff` drops the multisample target so the pass renders straight into the canvas); Canvas2D ignores it. `Options.DevTools`/`RecordReplay` mount `internal/devpanel` (backtick key) in builds tagged `arr_dev`, drawn inside the canvas; the browser-specific part is the Blob download and file picker behind `RecordingPort`. |
| `platform/native/` | Native host + streaming renderer for Windows/amd64, on wgpu-native and GLFW. `native.Run`/`RunDriver` take `Options` for window, VSync, icon, logging, and `FrameMetrics`. Reports CPU tick/render time (split from the presentation-queue wait, so the vblank block never reads as CPU work), input latency, Go memory, and particle buffer bytes; GPU timing is unavailable (unsafe completion callback, no per-pass timestamps in the binding) and retained batches are unsupported. The host locks its OS thread; GLFW callbacks only queue input, applied through `Driver.RunTick` before rendering. Alt+Enter toggles borderless fullscreen and the backtick key toggles the `internal/devpanel` overlay in builds tagged `arr_dev` (`Options.DevTools`/`RecordReplay`/`GameFactory`); its `RecordingPort` writes timestamped `.arrp` files into `recordings/` and loads the newest. `internal/sysprobe` feeds the panel's Perf tab with process-vs-system RAM, VRAM (DXGI `QueryVideoMemoryInfo` matched to the wgpu adapter), and GPU utilization (PDH engine counters), each behind a per-OS prober — an OS without one (the WASM host) leaves the rows hidden. The renderer uses the shared GPU tessellation/WGSL core for solid/textured/sprite/text streams and direct particle compute with MSAA (`Options.MSAA` takes a `render.MSAAMode`: the zero value is 4x, `render.MSAAOff` drops the multisample surface entirely). A process-global oto sink decodes manifest WAV/MP3/OGG clips, reconciles cues/tracks (loop/gain/pan/fade), and suspends/resumes on focus changes. |
| `cmd/` | Demos and tools. Game code lives here, not in engine packages. |

### Runtime Tick Semantics

- Platforms drive time through `Driver.AdvanceTime(elapsed)`. `Driver` owns `dt` = `1/SimulationHz`.
- Interactive hosts use `runtime.FrameTimer` for first-frame/activation timing, and `Driver.RunTick` for the canonical order: viewport → controls → input → advance → audio.
- `AdvanceTime` order: consume dev controls → apply reset/rewind → convert wall time by speed → fixed steps with a catch-up clamp → run each step.
- `Driver.Step()` runs exactly one fixed tick, no wall clock — use it for tests and replay.
- Per-step order: capture rewind snapshot → snapshot input → `Game.Update` → auto `World.Step` (locomotion → acceleration → ballistics → integrate → bounce → collision → damping, unless `ctx.HoldSimulation()`) → optional `PostStep` → advance tick → clear pointer frame → `mapper.EndFrame()` → prune input buffer.
- Hosts drain audio with `Driver.ConsumeAudio` from the simulation loop, after `AdvanceTime`/`Step` — never from render.
- Input applies before `AdvanceTime`/`Step`, via `ApplyInput`, stamped with the current tick.
- **Do not reorder these steps.** Determinism and rewind depend on this pipeline.
- `StateSlot[T]` snapshots by value: `T` must be a pure value type (no slices/maps/pointers), or implement `Clone() T` (`runtime.Cloneable`) to deep-copy reference fields — runs on both capture and restore.

### Boot Splash

- `Driver.RunSplashFrame` owns the phase before tick 0. Real-time hosts (`platform/wasm`, `platform/native`) offer it viewport/input and let it advance/render before their tick loop; offline (`platform/raster`) and replay paths skip it and reach tick 0 immediately. It runs on wall clock only — never advances `d.tick`, simulation, recorder, or checksum state. A skippable segment advances only on activation input (`input.EventButtonDown`).
- Playlist: optional engine segment + manifest `"splash"` segments. The engine segment renders the compiled brand animation (`internal/enginesplash`, built offline by `arasy svg`) as texture-free vector commands, no manifest entry or upload (`Config.DisableDefaultSplash` opts out). Game segments are texture flip-books from the manifest's `"splash"` array (`cell_w`/`cell_h`/`columns`/`frames`/`rate`/`min_time`/`fade`/`skippable`), packed with `arasy atlas <frames-dir|gif> -o out.png`.
- `--no-engine-splash` is a host-only opt-out that keeps manifest segments (`arasy dev` defaults it ON). `--no-loading` controls only the HTML download indicator; `--no-splash` controls only runtime segments. `--boot-background` sets the web phase-0 color; `Config.BootBackground` sets runtime splash color and falls back to `Config.Clear`.
- `HoldUntilReady` loading-screen behavior is not yet implemented; every segment today is bounded by `MinTime` only.

### ECS Pattern — Adding a Component

1. Define the struct in `scene/components.go`.
2. Add a `store[ComponentType]` field to `World`.
3. Add `Set`/getter/`Remove` methods delegating to the store, following the `SetX(EntityID, T)` + `X(EntityID) (T, bool)` convention exactly — structural guards discover components by reflecting over this convention.
4. Clone in `World.Clone()` (guard: `TestCloneCoversAllComponentStores`).
5. Remove in `World.Despawn()` (guard: `TestDespawnCoversAllComponentStores`).
6. Include in `World.Checksum()` and bump `ChecksumComponentStores` (a structural guard test fails if you forget).

### Simulation Determinism
- No `map` iteration in the simulation path. Component stores stay ordered by `EntityID`. The grid broad-phase (`collision.go`) is the reference pattern for deterministic aux structures.
- Use `ctx.Rand` in `Update`, never the global RNG. Render must never draw random values — frames are not ticks.
- `World.Checksum()` and the driver debug log verify sync.

### Test Structure
- Follow `docs/testing.md`. Name files by behavior, not one broad `*_test.go` per package.
- `package foo_test` by default; `package foo` only when testing internals.
- Helpers call `t.Helper()`; move to a shared file once used across several test files.
- Benchmarks go in dedicated `*_benchmark_test.go` files.
- `internal/simfuzz` provides twelve fuzz targets for simulation determinism (see `docs/testing.md`).

## Render Interpolation

Built-in for official platforms (WASM, offline raster, future native).

- `runtime.Config.RenderInterpolation` (default `false`) enables it; `runtime.RenderContext.Alpha` exposes the `[0, 1)` tick fraction.
- `scene.World.RenderCommandsInterpolated(alpha)` returns interpolated commands; `SyncRenderState()` captures the previous tick's transforms.
- Presentation-only: never enters `Checksum()`, replay, rewind snapshots, or recordings. `SetTransform` resyncs immediately to avoid smear on teleport/reset.
- Only Position/Rotation/Scale interpolate (`Vec2.Lerp`, `math2d.Lerp`); debug overlays (colliders, grid, velocity) always show current sim state.
- Games opt in via `ctx.World.RenderCommandsInterpolated(ctx.Alpha)` in `Render`.

## Retained Render Batches

`render.DrawSpriteBatch`/`render.DrawTextBatch` draw content published once
through `runtime.BatchPort` (`UpdateSprites`/`DeleteSprites`/`UpdateText`/
`DeleteText`, shared by `Game.Init`/`Update`/`Render`) instead of every frame
like `DrawImage`/`DrawText` streaming.

- Render-only: never enters `Checksum()`, snapshots, replays, or recordings; rewind neither drops nor undoes a publication.
- A batch command is a painter-order boundary owning one layer and one clip. A replace is a full-content swap (no partial update in v1); an empty replace keeps an empty batch — only `Delete*` removes it. A command referencing a missing/deleted batch is skipped with a one-time warning.
- Culling is conservative and whole-batch: world bounds computed on update, inflated by animation at draw time; no per-instance culling, compute, or reordering.
- GPU animation (`SpriteAnimation` flip-book/phase/bounce, `SpritePulse`, `SpriteBatchAnimation`) is visual-only, mirrored across raster/Canvas2D/WGSL by `internal/renderbatch` — no arbitrary shaders, no gameplay state.
- WebGPU reuses the streaming storage-buffer pipelines; stable content uploads zero bytes per frame. Raster is the reference; Canvas2D is dev-only and caps batch size (see `platform/wasm/`).
- Retained sprites suit stable/rarely-changing content (crowds, scenery, HUD); dynamic ECS sprites stay on the streaming path.

## Replay Watch Mode

- `ControlReplayPlay`/`Pause`/`Step` advance the driver from the recorded journal (`playbackSteps`) instead of stepping live — watching a recording never forks the timeline. Contrast `ControlPlay`/`ControlStep`, the explicit "take control" gesture, which forks from a rewound tick as usual. `ControlScrubToTick`/`ControlBack*` reposition without leaving watch mode.
- Playback auto-pauses at `MaxRecordedTick` and stays in watch mode; speed and the catch-up clamp match live play. Cues are audible during playback and muted during scrub (already heard live); see `docs/audio.md`. `DriveState.ReplayMode` exposes the mode to hosts.
- `runtime.ReplayForViewing` reconstructs like `Replay` but keeps the keyframe recorder populated, returning the driver paused, in watch mode, at the final tick — use it when handing a loaded recording to an interactive host. `runtime.Replay` stays recorder-free for verification/tests. Both leave the driver able to emit cues afterward; only ticks reconstructed during loading stay silent.

Full guide: `docs/scrubbing.md` ("Modo assistir" section, Portuguese).

## Design Constraints
- Engine packages stay independent of app-specific rules.
- Prefer small, explicit APIs; add complexity only under real project pressure.
- Render commands stay backend-agnostic. Measure before optimizing.
- Prefer the standard library. Engine packages limit third-party dependencies to `golang.org/x/image`, where already used. CLI tooling under `internal/cli` may use `spf13/cobra`.

## Float Discipline

Sim path (`scene/`, `runtime/`, `math2d/`): only `+ - * /` and `math.Sqrt`.
`forbidigo` forbids transcendentals (see `.golangci.yml`).

**FMA:** arm64 fuses `a*b + c`. Mitigation: cast the product, `float64(a*b) + c`.
```go
func (v Vec2) Dot(o Vec2) float64 {
    return float64(v.X*o.X) + float64(v.Y*o.Y)
}
```
Review sum-of-products expressions for an explicit `float64()` on the product operand.

## CI Gates

| Job | Checks |
|---|---|
| `quality` | actionlint, golangci-lint (forbidigo), govulncheck |
| `test` | `gofmt`, `go vet`, `go test ./...` (x86-64) |
| `no-default-font` | `go test -tags arr_no_default_font ./...` + js/wasm build+vet with the tag (embedded default font compiled out) |
| `race` | `go test -race ./...` |
| `windows` | `go test ./...` (Windows compat) |
| `wasm` | `GOOS=js go build ./...` + `go vet ./...` + sim-path tests via `go_js_wasm_exec` (incl. `examples/...` golden replays) + Canvas2D visual golden; WebGPU golden remains a local/browser gate |
| `arm64` | `go test ./...` (native arm64) |

`wasm` + `arm64` form the cross-arch determinism net. Golden checksum
(`scene/testdata/checksum_golden.json`) must match x86-64, arm64, and wasm —
regenerate in a dedicated commit on x86-64 when the format changes. Golden
replays (`examples/*/testdata/*.arrp`) run on all four Go architectures
(x86-64, windows, arm64, wasm) via `go test ./examples/...`, extending the
same net to the full driver+input+UI+audio+game pipeline.

## Embedded Default Font

The engine embeds Go Mono (BSD-3, `golang.org/x/image/font/gofont/gomono`) as
the default font for `FontID(0)`; text layout still uses `render.MeasureText`
(monospace, 0.6 advance, deterministic).

    Arasy build --no-default-font

excludes the ~140 KB TTF, falling back to the built-in 7×13 bitmap font.
Raster goldens with `FontID(0)` text exist in two variants:
`platform/raster/testdata/golden/` (embedded font) and
`.../golden/bitmap/` (opt-out) — regenerate the second with

    go test -tags arr_no_default_font ./platform/raster -run TestGolden -update

Both sets stay pixel-exact; the `no-default-font` CI job guards the opt-out.

## Reference Docs
- `README.md` -- capabilities, quick start, demo instructions
- `docs/runtime.md` -- runtime guide (tick semantics, Driver API, rewind)
- `docs/audio.md` -- audio guide (cues, tracks, host contract, rewind/scrub policy)
- `docs/ui.md` -- UI guide (themes, coord spaces, interaction)
- `docs/scrubbing.md` -- bidirectional time scrubbing (replay, timeline forking, checksum)
- `docs/testing.md` -- test organization conventions
- `docs/animation.md` -- `anim.Timeline`/`ui.Sequence`/`render.Builder` groups, clock ownership
- `docs/vector-graphics.md` -- `render/vector` path/stroke/motion API and the `arasy svg` compiler
