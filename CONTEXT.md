# Arasy Project Context

## Purpose

Arasy is an experimental 2D engine written in Go.

The project values clarity, deterministic simulation, low coupling, and small
inspectable packages. It is not a commercial engine. The project uses demos and
tools to test engine decisions.

## Current architecture

```text
math2d -> engine vocabulary -> scene/runtime -> platform hosts -> cmd/examples
```

The main packages have these roles:

- `math2d` provides pure 2D math types.
- `render` defines backend-agnostic drawing commands.
- `input`, `ui`, `audio`, `asset`, and `particle` define engine vocabulary.
- `scene` owns entities, components, and simulation systems.
- `runtime` owns the game contract, fixed ticks, input, rewind, replay, and
  frame production.
- `platform/raster`, `platform/wasm`, and `platform/native` implement hosts.
- `cmd/` and `examples/` contain games, demos, and tools.

## Stable boundaries

- Platforms drive games through `runtime.Driver`.
- Games receive the world through runtime contexts.
- Runtime code carries resolved asset data but does not parse backend formats.
- Render commands do not depend on a renderer or window system.
- Simulation state and presentation state have different lifetimes.
- The raster renderer is the reference for geometry and color behavior.

## Deterministic model

The driver owns the fixed simulation timestep. A normal step applies input,
runs `Game.Update`, advances the world unless the game holds simulation, runs
post-step work, advances the tick, and clears per-tick input state.

Simulation code must use controlled state and deterministic ordering. Render
code may interpolate presentation values, but it must not change simulation
state. Checksums, snapshots, recordings, and replay cover simulation state, not
render-only state.

## Data model

The engine favors simple components and systems that operate on component data.
The `scene.World` owns entity lifetime and component stores. Component stores
remain ordered when order affects simulation results. New components must follow
the store, clone, despawn, checksum, and structural-guard conventions.

## Rendering model

Games describe output with render commands. Hosts interpret those commands.
Camera projection, sprite planes, height presentation, depth order, retained
batches, particles, and interpolation are presentation concerns. Their detailed
contracts live in the rendering and 2.5D guides.

## Project vocabulary

- **Driver**: the `runtime.Driver`, which owns the fixed timestep and the
  game contract.
- **World**: the `scene.World`, which owns entity lifetime and component
  stores.
- **Component store**: the ordered container inside the world that holds one
  component type.
- **Render command**: a backend-agnostic drawing instruction that a game
  produces and a host interprets.
- **Simulation state**: state that affects simulation results and is covered
  by checksums, snapshots, recordings, and replay.
- **Presentation state**: state that only affects output. It is never part of
  simulation results.
- **Golden**: a recorded reference image or fixture that a test compares
  against current output.
- **Tick**: one fixed simulation step owned by `runtime.Driver`.
- **Frame**: one presentation produced by a host. A frame is not a tick.
- **Replay**: deterministic reconstruction from recorded input.
- **Scrub**: non-destructive movement through a recorded timeline.
- **Render-only**: presentation state that does not affect simulation results.
- **Host**: a platform package that drives the runtime and presents frames.

## Documentation map

- `AGENTS.md` contains agent rules and validation gates.
- `docs/en/architecture.md` describes package responsibilities and layering.
- `docs/en/runtime.md` describes the runtime contract.
- `docs/en/testing.md` describes test organization and test quality.
- `docs/en/development.md` lists development commands and platform gates.
- `docs/en/render-batches.md` describes retained batches and interpolation.
- `docs/en/boot-splash.md` describes boot-time presentation.
- `docs/en/scrubbing.md` describes replay, rewind, and timeline forking.
- `docs/en/2-5d.md` describes projection, planes, height, and depth order.

English guides live under `docs/en/`; Portuguese guides live under
`docs/pt-br/` and use the same filename. `docs/en/distribution.md` is currently
English-only.

## Planning

The GitHub project view is the source of truth for the roadmap. Do not add a
local roadmap to this context file. A plan may change the architecture only
after its current code, documentation, and validation gates are checked.
