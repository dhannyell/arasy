# AGENTS.md

Instructions for IA agents working in this repository.

## Commands

```powershell
# Tests
go test ./...                          # all tests
go test ./scene -run TestStep          # single test
go test ./scene -run="^$" -bench=. -benchmem  # benchmarks

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

# Golden files (regenerate after intentional visual changes)
go test ./platform/raster -run TestGolden -update
go test ./platform/web/canvas/internal/dto -run TestGolden -update
```

Golden PRs must explain the visual/DTO change; unexplained diffs are regressions.

## Architecture

```
Driver.Step/AdvanceTime -> Game.Update -> World.Step/PostStep -> render commands -> platform renderer
```

### Layers

```
foundation:     math2d
vocabulary:     render, input, ui, audio
simulation:     scene
orchestration:  runtime
hosts:          platform/raster, platform/web/canvas, platform/wasm
adapters:       platform/web/canvas/internal/dto
apps:           cmd/
```

**Dependency direction** (never reverse):
```
cmd/demos -> platform/* -> runtime -> {scene, input, ui, render, audio, asset} -> math2d
asset -> render -> math2d
platform/* -> asset/pixel -> asset
```

### Rules
- `math2d`, `render`, `input`, `ui`, `scene`, `runtime` never import `platform/*`.
- `runtime` is the sole behavioral API for platforms/games. Platforms call `runtime.Driver` methods only; never `scene.World` directly.
- `runtime` is backend-free: no JSON, HTTP, images, browser concepts. Manifest parse lives in `asset`; `runtime` only transports the resolved `asset.Manifest`.
- Games implement `runtime.Game`; World arrives via `UpdateContext`.

### Package Responsibilities

| Package | Role |
|---|---|
| `math2d/` | Pure 2D math (`Vec2`, `Rect`). Zero engine deps. |
| `render/` | Backend-agnostic commands, `Color`, `MeasureText`, `Camera`, `CommandSink` and stable `Each` dispatch; `DrawText.Font` selects a per-run `FontID`, and `DrawImageQuad` maps a texture onto a convex projective quad. Raster is the geometry/color reference; Canvas2D hosts approximate quads with an affine grid. |
| `asset/` | Data-driven texture/sprite/font/audio declarations from layered `fs.FS` manifests (last layer wins). `render.TextureID` and `render.FontID` are stable per-run, resolved by name, and never persisted. `asset/pixel` owns shared image decode; `asset/typeface` owns deterministic SFNT/OpenType metrics and parsing; `runtime` carries only the resolved `asset.Manifest`. |
| `audio/` | Backend-agnostic audio vocabulary: clip IDs, one-shot cues, continuous tracks, and rewindable track state. Hosts decode/play; runtime only transports intent. |
| `scene/` | ECS world + simulation tick. `EntityID` (uint32). Step pipeline: acceleration -> integrate -> bounce -> grid broad-phase -> contact resolution -> damping. `World.Checksum()` for determinism. |
| `input/` | `Source`/`Action`/`Binding`/`Mapper`/`Buffer` + pointer types. `Mapper.EndFrame()` called by driver at tick end. |
| `ui/` | Immediate-mode: Button, Panel, Label, etc. Three coord spaces. `ui.State` owned by driver (captured in rewind). |
| `runtime/` | Orchestration hub. Owns `Game` contract, `Driver` (fixed timestep, dev controls, rewind), `FrameTimer`, `RunTick`, `StateSlot[T]`, and `ConsumeAudio`. Defines *what a tick is* -- platforms must not duplicate. |
| `loop/` | Fixed timestep step planner. Runtime clock uses it; platforms never choose `dt`. |
| `platform/raster/` | Software renderer -> `image.RGBA`. Offline host (PNG/GIF via `Driver`). |
| `platform/wasm/` | Browser host (`GOOS=js`): `requestAnimationFrame`, WebGPU (default) + Canvas2D fallback. WebGPU uploads manifest textures and per-`(FontID, pixelSize)` `r8unorm` glyph atlases (`FontID(0)` remains the built-in bitmap font), generates texture mip chains at boot, and uses linear anisotropic image sampling; Canvas2D approximates projective image quads with a grid and paints external fonts through the browser FontFace API (monospace until the FontFace loads or when it is unavailable). Dev panel (backtick key) for debug controls. |
| `cmd/` | Demos and tools. Game code lives here, not in engine packages. |

### Runtime Tick Semantics

- Platforms drive time via `Driver.AdvanceTime(elapsed)`. Driver owns `dt` = `1/SimulationHz`.
- Interactive hosts use `runtime.FrameTimer` for first-frame/activation timing and `Driver.RunTick` for the canonical viewport → controls → input → advance → audio order.
- `AdvanceTime`: consume dev controls -> apply reset/rewind -> convert wall time x speed -> fixed steps with catch-up clamp -> run each step.
- `Driver.Step()`: exactly one fixed tick, no wall clock (for tests/replay).
- Per step: capture rewind snapshot -> snapshot input -> `Game.Update` -> auto `World.Step` (unless `ctx.HoldSimulation()`) -> optional `PostStep` -> advance tick -> clear pointer frame -> `mapper.EndFrame()` -> prune input buffer.
- Hosts drain audio cues/tracks with `Driver.ConsumeAudio` from the simulation loop after `AdvanceTime`/`Step`, never from render.
- Input applied *before* `AdvanceTime`/`Step` via `ApplyInput`, stamped with current tick.
- **Do not reorder.** Determinism and rewind depend on this pipeline.
- `StateSlot[T]` snapshots by value: T must be a pure value type (no slices/maps/pointers).

### ECS Pattern -- Adding a Component

1. Define struct in `scene/components.go`
2. Add `store[ComponentType]` field to `World`
3. Add `Set`/getter/`Remove` methods delegating to store
4. Clone in `World.Clone()`
5. Remove in `World.Despawn()`
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

## Render Interpolation

The engine provides built-in render interpolation for official platforms (WASM,
offline raster, future native). The platform/web/canvas debug tool does NOT
receive interpolation.

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

## Design Constraints
- Engine packages stay independent of app-specific rules.
- Prefer small explicit APIs; add complexity under project pressure.
- Render commands stay backend-agnostic. Measure before optimizing.
- Stdlib preferred; only dep is `github.com/coder/websocket`.

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
| `race` | `go test -race ./...` |
| `windows` | `go test ./...` (Windows compat) |
| `wasm` | `GOOS=js go test ./scene/... ./runtime/... ./math2d/... ./ease/...` via `go_js_wasm_exec` |
| `arm64` | `go test ./...` (native arm64) |

`wasm` + `arm64` = cross-arch determinism net. Golden checksum (`scene/testdata/checksum_golden.json`) must match x86-64, arm64, and wasm. Regenerate in a dedicated commit on x86-64 when format changes.

## Reference Docs
- `README.md` -- capabilities, quick start, demo instructions
- `docs/runtime.md` -- runtime guide (tick semantics, Driver API, rewind)
- `docs/audio.md` -- audio guide (cues, tracks, host contract, rewind/scrub policy)
- `docs/ui.md` -- UI guide (themes, coord spaces, interaction)
- `docs/scrubbing.md` -- bidirectional time scrubbing (replay, timeline forking, checksum)
- `docs/testing.md` -- test organization conventions
