# Agent Instructions

Read this file before changing repository files. Keep this file short. Read the
referenced documents when you need subsystem detail.

## Required

### Scope and safety

- Follow the user's requested scope.
- Preserve existing dirty changes.
- Do not reset, discard, or overwrite unrelated work.
- Do not change public behavior when the request is documentation-only.
- Do not commit, push, or open a pull request unless the user asks.
- Keep comments concise. Explain why, not what.
- Use simple ASD-STE-style comments. Add context instead of describing code.
- Add a doc comment to every exported function, type, constant, and variable.

### Architecture

- Keep dependencies in this direction: `cmd` -> `platform` -> `runtime` ->
  engine packages -> `math2d`.
- Do not import `platform/*` from `math2d`, `render`, `input`, `ui`, `scene`,
  or `runtime`.
- Use `runtime.Driver` as the behavioral API for games and platforms.
- Keep `runtime` free of backend, browser, HTTP, image, and JSON concepts.
- Keep render commands backend-agnostic.
- Treat `render.PostChain` as render-only vocabulary with the seven fixed
  effects: `Vignette`, `ColorGrade`, `Pixelate`, `ChromaticAberration`,
  `Bloom`, `Shockwave`, and `LensRain`.
- Raster is the canonical post-effect behavior; WebGPU and native consume the
  shared WGSL shaders, while Canvas2D approximates or skips effects per its
  documented capability table.
- Put game code in `cmd/` or `examples/`, not in engine packages.
- See [architecture](docs/en/architecture.md) for package responsibilities.

### Determinism

- Let `runtime.Driver` own fixed-timestep execution.
- Do not reorder the driver or world-step pipeline.
- Use `ctx.Rand` in simulation code. Do not use random values during render.
- Do not use unordered map iteration in simulation results.
- Mark a provably order-independent map loop with the
  `//arasy:order-insensitive` directive on the comment line above it, and
  state the reason in the same comment. The directive is per loop; the
  `//arasy:render` directive is per file.
- Keep presentation state out of checksums, snapshots, recordings, and replay.
- Leave `Config.GameID`/`GameVersion` empty in tests and fixtures; shipped
  games set them so recordings carry provenance.
- Treat `StateSlot[T]` reference fields as requiring `Clone() T`.
- Use `dmath` for transcendental math in simulation code; `math`
  transcendentals vary per architecture and break replay and checksums.
- Mark render-only files that need `math` transcendentals with the
  `//arasy:render` directive before the package clause.
- `arasy build` refuses non-reproducible simulation code: `math`
  transcendentals, map iteration, randomness outside `ctx.Rand`, and
  wall-clock reads. A direct `go build` does not check them.
- Read [runtime](docs/en/runtime.md) and [scrubbing](docs/en/scrubbing.md) for
  the full replay contract.

### Code size and structure

- Inspect existing code before adding a function or helper.
- Change an existing function when it is the correct extension point.
- Keep code small, decoupled, and easy to read.
- Do not add defensive branches or abstractions without a concrete failure mode
  or a clear maintenance benefit.

### Adding an ECS component

1. Define the component in `scene/components.go`.
2. Add its store to `scene.World`.
3. Add `Set`, getter, and `Remove` methods.
4. Clone the store in `World.Clone()`.
5. Remove the component in `World.Despawn()`.
6. Add simulation state to `World.Checksum()` and update the store count.

### Verification

- Format changed Go files with `gofmt`.
- Run `go vet ./...`.
- Run `go test ./...`.
- Run `golangci-lint run ./...` before handoff or commit.
- If the change affects WebAssembly, run the WASM build gate.
- Run browser goldens when browser behavior or visual output changes.
- Regenerate a golden only after an intentional visual or simulation change.
- Explain every intentional golden or replay fixture change.

### Float discipline

- In `scene/`, `runtime/`, and `math2d/`, use only `+`, `-`, `*`, `/`,
  `math.Sqrt`, and `math.Mod` for simulation math; use `dmath` when a
  transcendental is unavoidable.
- Cast the product before a sum of products when cross-architecture results
  must match: `float64(a*b) + c`.
- A product that feeds a sum includes a product held in a variable.
  Assignment does not round, and the compiler can fuse across statements,
  so write `v := float64(a*b)` when `v` later meets a `+` or `-`.
- Follow the detailed policy in [testing](docs/en/testing.md).

## Recommended

- Prefer small, explicit APIs.
- Measure before optimizing.
- Name tests after behavior.
- Use `package name_test` unless internal access is required.
- Give every new unit test a clear reason before adding it.
- Do not add shallow tests only to increase coverage.
- It is acceptable to leave simple files without tests when another test covers
  their behavior indirectly.
- Keep stable project context in [CONTEXT.md](CONTEXT.md), not here.
- Keep public guides in matching Portuguese and English files.

## Reference

- [Project context](CONTEXT.md)
- [Architecture](docs/en/architecture.md)
- [Development commands and gates](docs/en/development.md)
- [Runtime](docs/en/runtime.md)
- [Testing](docs/en/testing.md)
- [Scrubbing and replay](docs/en/scrubbing.md)
- [2.5D projection](docs/en/2-5d.md)
- [2.5D gaps survey](docs/en/2-5d-gaps.md)
- [Animation](docs/en/animation.md)
- [Render batches and interpolation](docs/en/render-batches.md)
- [Boot splash](docs/en/boot-splash.md)
- [Vector graphics](docs/en/vector-graphics.md)
