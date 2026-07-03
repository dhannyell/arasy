# AGENTS.md

This file provides guidance for IA Agents when working with code in this repository.

## Commands

```powershell
# Run all tests
go test ./...

# Run tests for a single package
go test ./scene

# Run a specific test
go test ./scene -run TestStep

# Run benchmarks
go test ./scene -run="^$" -bench="." -benchmem

# Save benchmark report to benchmarks/ (defaults to -pkg ./scene)
go run ./cmd/benchsave

# List registered benchmark suites (cmd/benchsave/suites.go is the hub's
# suite registry — add new families there, not via one-off -pkg flags)
go run ./cmd/benchsave -list-suites

# Run one named suite instead of passing -pkg/-bench manually
go run ./cmd/benchsave -suite runtime

# Run every registered suite (one report per suite)
go run ./cmd/benchsave -suite all

# Save a report and print/append a % comparison against the most recent
# previous report for the same -pkg (ns/op, B/op, allocs/op; negative % = faster/smaller)
go run ./cmd/benchsave -compare auto

# Compare against a specific previous report instead of auto-detecting one
go run ./cmd/benchsave -compare benchmarks/bench-scene-2026-06-28-211550.md

# Compare two already-saved reports without running any benchmarks
# (flags must come before the two report paths — stdlib `flag` stops parsing at the first non-flag arg)
go run ./cmd/benchsave -diff benchmarks/old-report.md benchmarks/new-report.md

# Same as above, but also save the comparison as its own markdown file
go run ./cmd/benchsave -diff -comparison-out benchmarks/compare.md benchmarks/old-report.md benchmarks/new-report.md

# Compare every report family in benchmarks/ at once (bench-scene, bench-runtime-seek, etc.),
# diffing each family's two most recent reports — no benchmarks are run
go run ./cmd/benchsave -compare-all

# Same as above, saved to a file too
go run ./cmd/benchsave -compare-all -comparison-out benchmarks/compare.md

# Offline render demo (PNG + frame-by-frame sequence in output/)
go run ./cmd/demos/football/raster

# Browser demos (WebSocket canvas at http://localhost:8080)
go run ./cmd/demos/football/webCanvas
go run ./cmd/demos/breakout/canvas
```

## Architecture

The core data flow is:

```
Driver.Step/AdvanceTime → Game.Update → World.Step/PostStep → render commands → platform renderer
```

### Layers

```
foundation:     math2d
vocabulary:     render, input, ui          (passive types + pure helpers)
simulation:     scene
orchestration:  runtime                    (Game contract, Driver, tick semantics, rewind)
hosts:          platform/raster, platform/web/canvas
adapters:       internal/render/adapter/web
apps:           cmd/
```

**Dependency direction** (never reverse these):

```
cmd/demos → platform/* → runtime → {scene, input, ui, render} → math2d
scene → render → math2d
internal/render/adapter/web → render
```

Rules:

- `math2d`, `render`, `input`, `ui`, `scene`, and `runtime` must never import `platform` packages.
- `runtime` is the only *behavioral* API for platforms and games. Platforms may
  import vocabulary types (`render.Command`, `input.Event`, `math2d`), but all
  behavior goes through `runtime.Driver` (`ApplyInput`, `Control`,
  `AdvanceTime`, `Step`, `Frame`) — platforms must not call `scene.World`
  methods directly.
- `runtime` must stay free of backend knowledge: no JSON, HTTP, image encoding,
  or browser concepts. Web-specific translation lives in `platform/web/canvas`
  and `internal/render/adapter/web`.
- Games implement `runtime.Game` and receive the `World` through
  `UpdateContext` — the World is never hidden from the game.

### Package Responsibilities

- **`math2d/`** — Pure 2D math primitives (`Vec2`, `Rect`). No dependencies on other engine packages.
- **`render/`** — Backend-agnostic render commands (`DrawRect`, `DrawCircle`, `DrawLine`, `DrawText`, `DrawArc`, `DrawPoint`), normalized `Color` (channels 0–1), and `Camera` with aspect modes (Stretch/Fit/Expand). Commands carry a `Layer()` int for draw ordering.
- **`scene/`** — Entity/component world and simulation systems. Entities are `EntityID` (uint32). Components are plain structs stored in per-type maps on `World`. `World.Step(dt, config)` runs the full simulation tick: acceleration → integrate → bounce inside bounds (per-side open mask + optional wall restitution via `StepConfig.Bounce`) → grid broad-phase → contact resolution (positional correction + restitution impulse) → damping. Contacts resolved in the last step are exposed via `World.Contacts()`. Also provides debug overlay command builders (`DebugCommands` with colliders/contacts/grid/velocities/IDs layers) and a deterministic `Checksum()` of positions+velocities.
- **`input/`** — Typed input system: `Source` (device+code), `Action` (semantic), `Binding` (source→action mapping with scale), `Mapper` (applies events to produce `State`), `Buffer` (per-tick event log), pointer types (`PointerState`, `PointerFrame`). `Mapper.EndFrame()` runs at the end of each tick (the runtime driver handles this).
- **`ui/`** — Immediate-mode UI: `Button`, `Panel`, `Label`, `ProgressBar`, click interaction, themes. Three coordinate spaces (`SpaceRef` reference canvas, `SpacePlayfield`, `SpaceScreen`). Interaction state (`ui.State`) is owned by the runtime driver and captured in rewind snapshots.
- **`runtime/`** — The orchestration hub and the engine's public behavioral API. Owns the `Game` contract (`Setup`/`Init`/`Update`/`Render`), `Driver` (fixed timestep pump, input application order, dev controls: pause/step/speed/reset/back), the rewind recorder (stores complete driver snapshots composed from World+driverInput+UI+game state), `StateSlot[T]` for global game state, and the runtime debug HUD. It defines *what a tick is* — that semantic must not be duplicated in platforms.
- **`loop/`** — Fixed timestep accumulator (`FixedStep`) and step planner (`Steps`). Runtime's `clock` uses `Steps` to convert elapsed wall time into N fixed simulation steps with a catch-up clamp. Platforms report elapsed time through `Driver.AdvanceTime(elapsed)` or use `Driver.Step()` for deterministic tick-driven flows; platforms must not choose simulation `dt` themselves.
- **`platform/raster/`** — `Renderer`: software renderer consuming `render.Command` slices into `image.RGBA` (no DrawText support yet, no alpha blending yet). `Run`: offline driver host producing PNG/GIF/frame sequences via `runtime.Driver`.
- **`platform/web/canvas/`** — HTTP + WebSocket host: streams JSON frames to a browser canvas, translates browser input/control messages into `input.Event`/`runtime.ControlCommand`, drives the `runtime.Driver` under a lock. **Positioning:** this is a development/visualization tool, not a distribution target — the official platform roadmap is WebAssembly first, then native Windows/Linux/macOS (see AD-4 in `plans/010-decisoes-arquiteturais.md`).
- **`internal/render/adapter/web/`** — Converts `render.Command` values to JSON DTOs for the web canvas (handles all command types including DrawText).
- **`cmd/`** — Runnable demos and tools. Game-specific code (e.g., `cmd/demos/football/`, `cmd/demos/breakout/`) lives here, not in engine packages.

### Runtime Tick Semantics

Platforms drive time with `Driver.AdvanceTime(elapsed)`. The Driver owns the
simulation `dt`: every simulation tick uses `1/SimulationHz`, regardless of the
host frame rate. `AdvanceTime` consumes dev controls, applies pending
reset/rewind commands, converts elapsed wall time × speed into fixed steps with
a catch-up clamp, then runs each step. Deterministic tests and future replay
code should use `Driver.Step()`, which executes exactly one fixed tick and does
not touch the wall-time accumulator.

Per step, the Driver captures a rewind snapshot, snapshots input state, calls
`Game.Update`, automatically calls `World.Step` unless the game called
`ctx.HoldSimulation()`, optionally calls `PostStep`, advances the tick, clears
the pointer frame, runs `mapper.EndFrame()`, and prunes the input buffer. Input
events are applied *before* `AdvanceTime`/`Step` via `ApplyInput` and stamped
with the current tick. Do not reorder this pipeline; determinism and rewind
depend on it.

`StateSlot[T]` snapshots by value: `T` must be a pure value type (no slices,
maps, or pointers), otherwise rewind snapshots alias mutable state.

### World / ECS Pattern

`World` holds all component maps. To add a new component type:
1. Define the struct in `scene/components.go`.
2. Add a `map[EntityID]ComponentType` field to `World`.
3. Add `Set`, getter, and `Remove` methods to `World`.
4. Clone the new map in `World.Clone()`.
5. Clean up in `World.Despawn()`.
6. Include it in `World.Checksum()` — the checksum must cover the full simulation state; equal checksums must mean equal states (AD-9 in `plans/010-decisoes-arquiteturais.md`).

### Simulation Determinism

Avoid `map` iteration order effects in simulation systems — use sorted entity
slices or spatial grids. The grid broad-phase (`collision.go`: sorted cells,
sorted per-cell entities, sorted pair list) is the reference implementation.
The per-entity systems in `movement.go` iterate maps directly and are safe only
because each iteration touches nothing but its own entity — keep that invariant
or switch to sorted slices. Never use unseeded/global RNG inside `Update`;
derive randomness from the tick if needed. `World.Checksum()` + the driver's
per-tick debug log exist to verify simulations stay in sync.

## Design Constraints

- Keep engine packages independent from application-specific rules.
- Prefer small, explicit APIs over large abstractions; add complexity only under real project pressure.
- Rendering commands stay backend-agnostic.
- Performance is measured (via benchmarks in `benchmarks/`) before optimizing.
- The Go standard library is preferred; the only external dependency is `github.com/coder/websocket`.
- Tests document behavior, especially for math, camera, loop, raster, scene, runtime, and ui.

## Reference Docs

- `README.md` — current capabilities, quick start (`runtime.Game` example), demo instructions.
- `docs/runtime.md` / `docs/runtime-en.md` — runtime guide.
- `docs/ui.md` / `docs/ui-en.md` — UI guide.
- `docs/scrubbing.md` / `docs/scrubbing-en.md` — bidirectional time scrubbing guide (replay-based ScrubToTick, timeline forking, checksum verification).
- `plans/009-revisao-tecnica.md` — full technical review: known bugs, architectural risks, and prioritized roadmap.
- `plans/010-decisoes-arquiteturais.md` — architectural decision record (AD-1…AD-9): auto-Step, input-log replay, platform roadmap, checksum contract, and more.
- `plans/008-runtime-pump-decomposicao-driver.md` — approved implementation plan: Driver owns time (`AdvanceTime`/`Step`) + Driver decomposition into collaborators.
- `plans/011-visao-diferenciais.md` — strategic differentiators: "every bug is a replay file" thesis, headless mass simulation, observable-engine tooling, rollback netcode as long-term north star; explicit anti-bets (rendering as an end, visual editor).
