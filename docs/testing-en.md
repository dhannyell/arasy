# Test Structure

This document records Arasy's test organization convention. The goal is
to keep idiomatic Go tests close to the code without letting packages turn into
large catch-all test files.

## Principles

- Tests live in the same directory as the package they exercise.
- Test files are named by behavior or responsibility, not necessarily by
  mirroring a production file.
- Prefer several small, focused files over one large `*_test.go` file.
- Tests should document observable behavior and simulation decisions.
- Test helpers should be small, explicit, and local to the package.
- Benchmarks live in separate `*_benchmark_test.go` files.

## Test Package

Use `package foo_test` by default when a test can validate the package's public
API. This keeps coupling low and mirrors how real users consume the package.

Current examples:

```text
scene           -> package scene_test
render          -> package render_test
input           -> package input_test
math2d          -> package math2d_test
platform/raster -> package raster_test
```

Use `package foo` when the important contract depends on package-internal
state, or when the test is a deliberate characterization of an internal
implementation.

Current examples:

```text
runtime             -> package runtime
ui                  -> package ui
platform/web/canvas -> package canvas
```

This choice should be intentional. When adding a new test file to a mixed
package, follow the existing style for the behavior being tested.

## File Names

Name files by behavior area:

```text
collision_bounds_test.go
collision_detection_test.go
collision_grid_test.go
collision_resolution_test.go
driver_rewind_test.go
driver_ui_test.go
renderer_text_test.go
camera_aspect_test.go
```

Avoid vague names once a file grows beyond a few tests:

```text
collision_test.go
driver_test.go
renderer_test.go
misc_test.go
```

Those names are still acceptable for small packages, but split the file when it
starts mixing behavior families.

## Recommended Organization By Package

### `scene`

`scene` concentrates simulation, ECS, collision, debug, and checksum behavior.
Tests should be separated by system or contract:

```text
world_test.go
movement_test.go
step_test.go
checksum_test.go
collision_bounds_test.go
collision_detection_test.go
collision_grid_test.go
collision_resolution_test.go
collision_shapes_test.go
collision_helpers_test.go
render_test.go
debug_test.go
collision_benchmark_test.go
step_benchmark_test.go
```

When adding a new component to `World`, cover at least:

- set/get/remove;
- `Clone`;
- `Despawn`;
- `Checksum`, when the component is part of simulation state.

### `runtime`

`runtime` defines tick semantics, input, rewind, replay, and runtime UI. Tests
should be grouped by the driver contract:

```text
driver_camera_test.go
driver_step_test.go
driver_rewind_test.go
driver_state_test.go
driver_ui_test.go
driver_public_test.go
driver_characterization_test.go
input_test.go
replay_test.go
scrub_test.go
debug_test.go
seek_benchmark_test.go
```

Use explicit names when a test is a bug regression or a semantic
characterization. Comments are welcome when they explain the bug history or the
deterministic reason behind the expectation.

### `platform/raster`

The rasterizer has pixel-level tests. Separate them by primitive or composition
rule:

```text
renderer_core_test.go
renderer_rect_test.go
renderer_circle_test.go
renderer_line_test.go
renderer_arc_test.go
renderer_text_test.go
renderer_layer_test.go
renderer_helpers_test.go
run_test.go
renderer_benchmark_test.go
```

Keep pixel assertions in small helpers, but leave the render command visible in
the test. That makes visual failures easier to understand.

### `render`

Separate camera transforms, aspect ratio, colors, and commands:

```text
camera_transform_test.go
camera_aspect_test.go
color_test.go
command_test.go
```

### Small Packages

Packages such as `math2d`, `loop`, `input`, and `ui` can keep only a few files
as long as each file tells one clear story.

## Helpers

Use helpers when they remove repetition without hiding the test case.

Good examples:

```go
func assertVec2Near(t *testing.T, got, want math2d.Vec2)
func assertPixel(t *testing.T, got color.Color, want color.RGBA)
func spawnCollidableCircle(w *scene.World, pos math2d.Vec2, radius float64) scene.EntityID
```

Helper rules:

- call `t.Helper()`;
- place helpers in `*_helpers_test.go` when several files use them;
- keep helpers in the same file when only one test family uses them;
- avoid large DSLs that hide the state being assembled.

## Integration And Heavy Tests

Integration tests may stay in the package they exercise while they remain fast
and stable. If a test needs a server, WebSocket, real time, or a full game,
make that explicit in the name:

```text
scrub_integration_test.go
```

If those tests become slow or unstable, use a build tag:

```go
//go:build integration
```

Then the flow becomes:

```powershell
go test ./...
go test -tags=integration ./platform/web/canvas
```

Do not add build tags too early. While the full suite stays fast and stable,
keeping everything under `go test ./...` gives better feedback.

## Benchmarks

Benchmarks live in dedicated files:

```text
collision_benchmark_test.go
step_benchmark_test.go
seek_benchmark_test.go
renderer_benchmark_test.go
```

When a benchmark becomes an official family, register the suite in
`cmd/benchsave/suites.go` so `go run ./cmd/benchsave -list-suites` can find it.

## When To Split A File

Split a test file when any of these conditions appears:

- it mixes two or more behavior families;
- it grows to a few hundred lines and navigation becomes tiring;
- new tests are being added far away from the helpers or fixtures they use;
- the file name no longer describes the contents well.

When splitting, prefer moving whole tests without changing behavior. Then, as a
second step, extract helpers if the repetition is still clear.

## Verification Commands

Before finishing test changes:

```powershell
gofmt -w <changed files>
go test ./...
```

To check coverage for a specific package:

```powershell
go test ./render -cover
go test ./scene -cover
go test ./runtime -cover
```

## Fuzzing the simulation

`internal/simfuzz` provides twelve fuzz targets that drive a dedicated
`runtime.Game` (the "fuzz game") through a real `runtime.Driver`, exercising the
full input -> Driver -> World pipeline. Each target decodes a `Script` from the
fuzzer bytes (a total, deterministic decoder), spawns entities with the complete
scene component vocabulary, and replays a monotonic input event log. P2 also
drives the real Breakout UI/game path, while P5 is seeded with the three
committed golden recordings.

| Target | Property | What it checks |
|--------|----------|----------------|
| `FuzzDeterminism` | P1 | Two independent drivers, same script -> identical checksum at every tick |
| `FuzzReplayFidelity` | P2 | Record -> `VerifyRecording` -> replay checksum == live run |
| `FuzzRewindConvergence` | P3 | Advance to N, scrub back to K<N, scrub forward to N -> same checksum |
| `FuzzNumericSanity` | P4 | No NaN/Inf in any component of any live entity after any tick |
| `FuzzRecordingCodec` | P5 | `Recording` -> bytes -> `Recording` round-trips stably |
| `FuzzNoPanic` | P6 | Arbitrary (possibly invalid) input never panics the engine |
| `FuzzResetReproducibility` | P7 | `ControlReset` mid-run rebuilds the world deterministically; per-tick checksums after reset match a fresh run |
| `FuzzScrubBidirectional` | P8 | Repeated scrub jumps (N -> K1 -> K2 -> ... -> N) each land on the original checksum, exercising keyframe archive thinning |
| `FuzzSpeedIndependence` | P9 | `ControlSetSpeed` changes tick pacing, never per-tick state; different speed histories give identical checksums |
| `FuzzDespawnMidRun` | P10 | An input-triggered despawn happens exactly once, stays finite/deterministic, and converges across rewind |
| `FuzzSubstepTunneling` | P11 | A shaped fast-projectile/thin-wall scenario exercises substep CCD, stays finite/deterministic, and never crosses the wall |
| `FuzzForkAfterScrubDeterminism` | P12 | Scrub + new input forks the timeline and matches a clean run of the forked future, which must differ from the abandoned future |

P4 is the headline: input floats are forced finite by the decoder, so any NaN or
Inf it finds was produced by the simulation itself (e.g. extreme-but-finite
`Body.InvMass` or `Ballistics.DragK`), not injected.

One event record in three decodes to an `EventAxisChanged` on a bound gamepad
axis rather than a button edge. That is the only path on which the `Mapper`
consumes `Event.Value`, so it is also the only way the value bytes of a record
can affect the simulation. `TestAxisEventDrivesSimulation` proves the path is
live end to end.

### How they run

1. **As regression tests, for free**, on every `go test ./...` — the Go test
   framework executes the seed corpus of every `FuzzXxx` function plus the
   committed files under `testdata/fuzz/<Target>/`. Cost: milliseconds.

2. **As a real fuzzer**, under demand with `-fuzz`:

   ```powershell
   go test ./internal/simfuzz -fuzz FuzzDeterminism -fuzztime 300s
   go test ./internal/simfuzz -fuzz FuzzRewindConvergence -fuzztime 120s
   # etc.
   ```

3. **In CI**, through the `.github/workflows/fuzz.yml` workflow, daily and on
   demand (`workflow_dispatch`) — **not** on pull requests. Fuzzing with a fixed
   budget is a poor pull-request gate: it either blocks merges on a flaky
   timeout, or it is marked non-blocking and then nobody reads it. The
   pull-request signal already exists and is free: the seed corpus runs as part
   of `go test ./...`.

### Triage flow

1. A fuzz target fails -> the check helper calls `t.Fatalf`.
2. `dumpRepro` writes both a valid Go fuzz corpus entry (`.txt`) and a replayable
   `.arrp` under `testdata/repro/<Target>/`; the failure message names both.
   The Go fuzz runner may also persist its minimized input directly under
   `testdata/fuzz/<Target>/`.
3. Move the `.txt` input into the target's seed corpus directory
   (`testdata/fuzz/<Target>/`) and commit it with the matching replay under
   `testdata/replays/<Target>/` **before** any fix.
4. Fix the bug, then verify the repro file passes.
5. Commit both the repro file and the fix in separate commits so the regression
   is reviewable independently.

`TestReproRoundTrip` proves a `dumpRepro` artifact reloads and re-decodes to the
same scenario.

### Adding a new target

1. Define the check helper in a behavior-specific `*_test.go` file.
2. Add a `Fuzz*` wrapper in `fuzz_properties_test.go`.
3. Seed it through `addSimulationSeeds`, `addRecordingSeeds`, or a deliberately
   shaped target-specific seed.
4. Confirm discovery with `go test ./internal/simfuzz -list '^Fuzz'`.

New targets must never panic; the decoder is total.
