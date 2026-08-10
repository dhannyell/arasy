# Arasy
[![CI](https://github.com/dhannyell/arasy/actions/workflows/ci.yml/badge.svg)](...)

Arasy is an experimental 2D engine written in Go.

The project is focused on learning, clarity, deterministic simulation, and low coupling between core engine concepts and rendering backends. It currently provides a small foundation for 2D math, fixed timestep loops, render commands, a portable runtime driver, a raster renderer, and a lightweight scene/entity model.

Arasy is not trying to compete with commercial engines. The goal is to build a compact, inspectable engine from first principles using mostly the Go standard library.

## Status

Arasy is in early development.

Current capabilities:

- 2D vector and rectangle math.
- Fixed timestep helper.
- Camera with aspect-ratio modes (stretch, fit, expand).
- Backend-agnostic render commands: rectangles, circles, lines, arcs, points, images, and text, with layer-based draw ordering and manifest-selected TTF/OTF fonts.
- Runtime package with a portable `Game` contract, automatic fixed-timestep world stepping, input mapping, dev controls (pause, step, speed, reset), and optional rewind history.
- Backend-agnostic audio intents with deterministic one-shot cues, continuous tracks, and Web Audio playback in the web canvas host.
- Immediate-mode UI package with basic widgets, pointer interaction, theme support, and three coordinate spaces (reference canvas, playfield, screen).
- Raster renderer using Go's standard `image` packages, including proportional OpenType text rendering and alpha blending.
- WASM/WebGPU renderer with per-font, per-pixel-size glyph atlases; the built-in bitmap font remains the zero-value fallback.
- Scene world with entities and components: transform, circle/rect sprite, velocity, acceleration, damping, body (inverse mass + restitution), and circle/rect collider.
- Physics step pipeline: acceleration → integration → bounds bounce (per-side open mask + optional wall restitution) → spatial-grid broad-phase → contact resolution (positional correction + restitution impulses) → damping.
- Deterministic simulation: sorted broad-phase pairs and a per-tick world checksum for sync verification.
- Debug overlays: colliders, contacts, spatial grid, velocities, entity IDs, input state, and a runtime stats HUD.
- WebSocket canvas platform with live simulation, browser input, and dev controls including rewind (back, back-to-tick, back-seconds).
- Offline raster platform producing PNG, animated GIF, and frame-by-frame sequences.
- Playable demos: football, breakout, 2048, Rhythm, and the UI-focused manager demo.

## Project Structure

```text
math2d/
  Basic 2D math primitives such as Vec2 and Rect.

loop/
  Fixed timestep utilities (accumulator and step planner).

render/
  Backend-agnostic rendering types, colors, commands, and camera.

scene/
  Entity/component world, physics simulation systems (movement, bounds
  bounce, spatial-grid collision), debug overlay builders, and a
  deterministic world checksum.

input/
  Typed input mapping system (device codes → semantic actions), pointer
  state, and a per-tick event buffer.

ui/
  Immediate-mode UI widgets, themes, and pointer interaction helpers.

audio/
  Backend-agnostic audio vocabulary for clip IDs, one-shot cues, continuous
  tracks, and rewindable track state. Clip paths are declared in asset
  manifests.

asset/
  Layered data-driven manifests for textures, sprites, fonts, and audio.
  The typeface subpackage parses TTF/OTF and provides deterministic metrics.

runtime/
  Orchestration hub: platform-agnostic Game contract, Context, Driver,
  input/control plumbing, fixed timestep pump, frame production, dev controls,
  audio output, and optional rewind history. The only behavioral API platforms
  need to drive.

platform/raster/
  Software renderer that draws render commands into image.RGBA, plus an
  offline runner producing PNG/GIF/frame-sequence output.
  
platform/wasm/
    Web Assembly platform that draws render commands into browser canvas using
    WebGPU. Automatic fallback to software rendering if GPU is not available or 
    not supported.

cmd/demos/
  A collection of simple demos covering various engine capabilities for a quick start. 

cmd/benchsave/
  Cross-platform helper that runs Go benchmarks and saves a Markdown report.

docs/
  Language-separated guides under en/ and pt-br/, plus project assets.
```

## Requirements

- Go 1.26 or newer, based on the current `go.mod`.

## Quick Start

A game implements `runtime.Game`: configure once, populate a `scene.World`,
drive entities in `Update`, and return backend-agnostic render commands from
`Render`. The runtime steps the world automatically after `Update`; use
`ctx.HoldSimulation()` for menus or ready states, and implement
`runtime.PostStepper` to react to same-tick contacts.

```go
type Game struct {
	runtime.StateSlot[GlobalState]

	player scene.EntityID
}

type GlobalState struct {
	Score int
}

func (g *Game) Setup() runtime.Config {
	return runtime.Config{
		Camera: render.Camera{
			World:    math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 100, Y: 75}),
			Viewport: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 800, Y: 600}),
			Aspect:   render.AspectExpand,
		},
		Clear:        render.RGBA(0.05, 0.08, 0.10, 1),
		Bounds:       math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 100, Y: 75}),
		SimulationHz: 60,
		FPS:          60,
	}
}

func (g *Game) Init(ctx *runtime.UpdateContext) {
	g.player = ctx.World.Spawn()
	ctx.World.SetTransform(g.player, scene.Transform{
		Position: math2d.Vec2{X: 50, Y: 37.5},
		Scale:    math2d.Vec2{X: 1, Y: 1},
	})
	ctx.World.SetCircleSprite(g.player, scene.CircleSprite{
		Radius: 3,
		Color:  render.RGBA(0.9, 0.2, 0.15, 1),
		Layer:  10,
	})
}

func (g *Game) Update(ctx *runtime.UpdateContext, dt float64) {
	// Read input and update world components here.
}

func (g *Game) Render(ctx *runtime.RenderContext) []render.Command {
	return ctx.World.RenderCommands()
}
```

Embedding `runtime.StateSlot[T]` is optional. Use it when the game has global
state that is not tied to a single entity, such as score, lives, match phase, or
the last tick that triggered a visual effect. The runtime captures and restores
that state together with rewind history:

```go
g.State.Score++
```

Run the game through a platform package, such as `platform/raster` for offline
PNG/frame output or `platform/wasm` for an interactive browser canvas.

```go
//go:build js && wasm

package main

import (
	"log/slog"

	"github.com/dhannyell/arasy/cmd/demos/breakout"
	"github.com/dhannyell/arasy/platform/wasm"
	"github.com/dhannyell/arasy/runtime"
)

func main() {
	log := wasm.NewConsoleLogger(slog.LevelInfo)
	if err := wasm.Run(&breakout.Breakout{}, wasm.Options{
		// DevTools needs the arr_dev build tag: arasy dev adds it, arasy
		// build --devtools produces a dev build with the panel.
		DevTools:      true,
		Log:           log,
		RecordReplay:  true,
		ForceCanvas2D: false,
		GameFactory:   func() runtime.Game { return &breakout.Breakout{} },
	}); err != nil {
		log.Error("wasm host failed", "err", err)
	}
}
```

## Running Tests

```powershell
go test ./...
```

## Running The Offline Demo

Generate a static PNG and a frame-by-frame PNG sequence:

```powershell
go run ./cmd/demos/football/raster
```

Output files:

```text
output/renderdemo.png
output/frames/frame-0000.png
output/frames/frame-0001.png
...
```

The PNG is a static high-resolution render. The frame sequence is used to validate movement and rendering, and can be combined into a video with external tools such as `ffmpeg`.

Rhythm also has an offline smoke render:

```powershell
go run ./examples/rhythm/raster
```

The 2.5D showcase renders offline the same way: `examples/outpost` is an
isometric island with a retained tilemap, extruded volumes, and hopping units:

```powershell
go run ./examples/outpost/raster
```

For the interactive demo, build the WASM entry point and serve `output/wasm/rhythm`
with the local development server. Use `D`, `F`, `J`, and `K` for the four lanes;
left/right arrows adjust the deterministic calibration offset.

## Running The Browser Demos

Compile the demo for WASM:

```powershell
# Football: WASD or arrow keys move the controlled player
$env:GOOS='js'; $env:GOARCH='wasm'; go build -o output/wasm/football/app.wasm ./cmd/demos/football/wasm; Remove-Item Env:GOOS; Remove-Item Env:GOARCH                                                 

# Breakout: A/D or arrow keys move the paddle, Space launches the ball
$env:GOOS='js'; $env:GOARCH='wasm'; go build -o output/wasm/breakout/app.wasm ./cmd/demos/breakout/wasm; Remove-Item Env:GOOS; Remove-Item Env:GOARCH                                                 
```

Run the Wasm server, pointing it to the generated Wasm file.
```powershell
 go run ./cmd/wasmserve -dir .\output\wasm\breakout\ -title "Arasy wasm" -addr 127.0.0.1:8080   
```
Then open `http://localhost:8080` in your browser.
Both demos include a screen-space UI overlay (score panel, reset button) and
the dev control bar: pause, single-step, speed, rewind, and debug overlay
toggles (colliders, contacts, spatial grid, velocities, entity IDs, runtime
stats).

## Measuring Performance

For comprehensive documentation on benchmarking workflows, baselines, and interpreting results, see [`benchmarks/README.md`](benchmarks/README.md).

Quick start:

```powershell
# List benchmark suites
go run ./cmd/benchsave -list-suites

# Run one suite and compare against the baseline
go run ./cmd/benchsave -suite runtime -compare baseline

# Update the baseline after an intentional change
go run ./cmd/benchsave -suite runtime -profile decision -update-baseline
```

## Reproducible Math in Simulation

Replays, rewind, and checksums only work when every machine computes the
same bits. Standard-library transcendentals (`math.Sin`, `math.Exp`, ...)
delegate to platform libm implementations, so their last bits vary per
architecture. The `dmath` package provides reproducible replacements.

The rule for game code:

- If it runs in `Update` or `PostStep`, or mutates state that enters a
  `StateSlot`, it is simulation code: use `dmath`, never `math`
  transcendentals. Exact operations (`+ - * /`, `math.Sqrt`, `math.Mod`)
  are always fine.
- Render-only code may use `math` freely. If a whole file is render-only,
  mark it with the `//arasy:render` directive before the package clause.

The same heuristic covers three more hazards in simulation code:

- Map iteration: Go deliberately randomizes map order, so a result that
  depends on it cannot replay reliably. Iterate an ordered slice of keys
  instead. A loop that provably does not depend on iteration order (pure
  membership work, commutative writes) can carry the
  `//arasy:order-insensitive` directive on the comment line above it;
  state the reason in the same comment.
- Randomness outside `ctx.Rand` (`math/rand`, `math/rand/v2`,
  `crypto/rand`): it bypasses the recorded stream.
- Wall-clock reads (`time.Now`, `time.Since`, `time.Until`): simulation
  time is the fixed tick; presentation time arrives through `ctx.Time`.
  `time.Duration` remains allowed in an API; only the clock reads are
  forbidden.

`arasy build` enforces these rules and reports the recommended fix for each
finding (`dmath`, `ctx.Rand`, the simulation tick, or a directive).
`arasy doctor` reports the same findings as warnings. A direct `go build` does
not run these project checks, so the guarantee applies only to the `arasy`
build path. The `--skip-determinism-check` flag is available when a finding is
known to be safe.

## Design Goals

- Keep engine packages independent from application-specific rules.
- Prefer small, explicit APIs over large abstractions.
- Keep rendering commands backend-agnostic.
- Favor deterministic simulation where practical.
- Use tests to document behavior.
- Use the Go standard library whenever it is reasonable.
- Add complexity only when the project pressure justifies it.

## Documentation

- Runtime guide: [Portuguese](docs/pt-br/runtime.md) | [English](docs/en/runtime.md)
- UI guide: [Portuguese](docs/pt-br/ui.md) | [English](docs/en/ui.md)
- Math visual guide: [Portuguese](docs/pt-br/math2d-visual-guide.md) | [English](docs/en/math2d-visual-guide.md)
- Camera, world, and viewport: [Portuguese](docs/pt-br/camera-world-viewport.md) | [English](docs/en/camera-world-viewport.md)
- Bidirectional time scrubbing: [Portuguese](docs/pt-br/scrubbing.md) | [English](docs/en/scrubbing.md)
- Test structure conventions: [Portuguese](docs/pt-br/testing.md) | [English](docs/en/testing.md)
- Animation: [Portuguese](docs/pt-br/animation.md) | [English](docs/en/animation.md)
- Vector graphics and SVG compilation: [Portuguese](docs/pt-br/vector-graphics.md) | [English](docs/en/vector-graphics.md)
- 2.5D: tilt, planes, height, and depth order: [Portuguese](docs/pt-br/2-5d.md) | [English](docs/en/2-5d.md)
- 2.5D gaps survey: [Portuguese](docs/pt-br/2-5d-gaps.md) | [English](docs/en/2-5d-gaps.md)

## License

Arasy is licensed under the [MIT License](LICENSE).
