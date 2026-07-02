# FrameForge

FrameForge is an experimental 2D engine written in Go.

The project is focused on learning, clarity, deterministic simulation, and low coupling between core engine concepts and rendering backends. It currently provides a small foundation for 2D math, fixed timestep loops, render commands, a portable runtime driver, a raster renderer, and a lightweight scene/entity model.

FrameForge is not trying to compete with commercial engines. The goal is to build a compact, inspectable engine from first principles using mostly the Go standard library.

## Status

FrameForge is in early development.

Current capabilities:

- 2D vector and rectangle math.
- Fixed timestep helper.
- Camera with aspect-ratio modes (stretch, fit, expand).
- Backend-agnostic render commands: rectangles, circles, lines, arcs, points, and text, with layer-based draw ordering.
- Runtime package with a portable `Game` contract, fixed-timestep driver pump, input mapping, dev controls (pause, step, speed, reset), and optional rewind history.
- Immediate-mode UI package with basic widgets, pointer interaction, theme support, and three coordinate spaces (reference canvas, playfield, screen).
- Raster renderer using Go's standard `image` packages (text rendering not supported yet on this backend).
- Scene world with entities and components: transform, circle sprite, velocity, acceleration, damping, body (inverse mass + restitution), and circle collider.
- Physics step pipeline: acceleration → integration → bounds bounce → spatial-grid broad-phase → contact resolution (positional correction + restitution impulses) → damping.
- Deterministic simulation: sorted broad-phase pairs and a per-tick world checksum for sync verification.
- Debug overlays: colliders, contacts, spatial grid, velocities, entity IDs, input state, and a runtime stats HUD.
- WebSocket canvas platform with live simulation, browser input, and dev controls including rewind (back, back-to-tick, back-seconds).
- Offline raster platform producing PNG, animated GIF, and frame-by-frame sequences.
- Two playable demos: football and breakout.

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

runtime/
  Orchestration hub: platform-agnostic Game contract, Context, Driver,
  input/control plumbing, fixed timestep pump, frame production, dev controls,
  and optional rewind history. The only behavioral API platforms need to drive.

platform/raster/
  Software renderer that draws render commands into image.RGBA, plus an
  offline runner producing PNG/GIF/frame-sequence output.

platform/web/canvas/
  HTTP + WebSocket server that streams JSON frames to a browser canvas and
  translates browser input/control messages back into the runtime.

internal/render/adapter/web/
  JSON DTO conversion of render commands and camera for the web canvas.

debug/
  Standalone World snapshot recorder (the runtime keeps its own internal
  recorder for rewind).

cmd/demos/football/
  Top-down football demo: controlled player, ball physics, goals, score
  HUD, and camera shake.

cmd/demos/football/raster/
  Offline render of the football demo (PNG + frame-by-frame PNG sequence).

cmd/demos/football/webCanvas/
  Browser-based football demo with live simulation and debug visualization.

cmd/demos/breakout/
  Breakout demo: paddle input, brick collisions, menu/game-over states, and
  UI overlays, playable in the browser via cmd/demos/breakout/canvas/.

cmd/benchsave/
  Cross-platform helper that runs Go benchmarks and saves a Markdown report.

docs/
  Additional notes, guides, and the technical review/roadmap.
```

## Requirements

- Go 1.26 or newer, based on the current `go.mod`.

## Quick Start

A game implements `runtime.Game`: configure once, populate a `scene.World`,
advance the simulation in `Update`, and return backend-agnostic render commands
from `Render`.

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
	ctx.World.Step(dt, ctx.StepConfig)
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
PNG/frame output or `platform/web/canvas` for an interactive browser canvas.

```go
package main

import (
	"log/slog"
	"os"

	"github.com/dhannyell/frameforge/platform/web/canvas"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	if err := canvas.Run(&Game{}, canvas.Options{Addr: ":8080", Log: log}); err != nil {
		log.Error("server failed", "err", err)
		os.Exit(1)
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

## Running The Browser Demos

Start the WebSocket server for one of the demos:

```powershell
# Football: WASD or arrow keys move the controlled player
go run ./cmd/demos/football/webCanvas

# Breakout: A/D or arrow keys move the paddle, Space launches the ball
go run ./cmd/demos/breakout/canvas
```

Then open `http://localhost:8080` in your browser.
Both demos include a screen-space UI overlay (score panel, reset button) and
the dev control bar: pause, single-step, speed, rewind, and debug overlay
toggles (colliders, contacts, spatial grid, velocities, entity IDs, runtime
stats).

## Running Benchmarks

Run benchmarks directly:

```powershell
go test ./scene -run="^$" -bench="." -benchmem
```

Save a structured Markdown report:

```powershell
go run ./cmd/benchsave
```

By default, reports are written to:

```text
benchmarks/
```

Useful options:

```powershell
go run ./cmd/benchsave -pkg ./scene -bench DetectCircleOverlaps -name scene-collision
```

## Design Goals

- Keep engine packages independent from application-specific rules.
- Prefer small, explicit APIs over large abstractions.
- Keep rendering commands backend-agnostic.
- Favor deterministic simulation where practical.
- Use tests to document behavior.
- Use the Go standard library whenever it is reasonable.
- Add complexity only when the project pressure justifies it.

## Documentation

- Runtime guide: [Portuguese](docs/runtime.md) | [English](docs/runtime-en.md)
- UI guide: [Portuguese](docs/ui.md) | [English](docs/ui-en.md)
- Math visual guide: [Portuguese](docs/math2d-visual-guide.md) | [English](docs/math2d-visual-guide-en.md)
- Camera, world, and viewport: [Portuguese](docs/camera-world-viewport.md) | [English](docs/camera-world-viewport-en.md)

## License

FrameForge is licensed under the [MIT License](LICENSE).
