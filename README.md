# FrameForge

FrameForge is an experimental 2D engine written in Go.

The project is focused on learning, clarity, deterministic simulation, and low coupling between core engine concepts and rendering backends. It currently provides a small foundation for 2D math, fixed timestep loops, render commands, a portable runtime driver, a raster renderer, and a lightweight scene/entity model.

FrameForge is not trying to compete with commercial engines. The goal is to build a compact, inspectable engine from first principles using mostly the Go standard library.

## Status

FrameForge is in early development.

Current capabilities:

- 2D vector and rectangle math.
- Fixed timestep helper.
- Camera with aspect-ratio modes.
- Backend-agnostic render commands.
- Runtime package with a portable `Game` contract, driver, input mapping, dev controls, and optional rewind history.
- Immediate-mode UI package with basic widgets, pointer interaction, and theme support.
- Raster renderer using Go's standard `image` packages.
- Filled/stroked rectangles.
- Filled/stroked circles.
- Lines, arcs, and point markers.
- Basic scene world with entities and components.
- Transform, circle sprite, and velocity components.
- Simple integration system for linear movement.
- PNG render demo with optional frame sequence output.

## Project Structure

```text
math2d/
  Basic 2D math primitives such as Vec2 and Rect.

loop/
  Fixed timestep utilities.

render/
  Backend-agnostic rendering types, colors, commands, and camera.

platform/raster/
  Software renderer that draws render commands into image.RGBA.

platform/web/canvas/
  WebSocket server that streams render commands to a browser canvas.

scene/
  Minimal entity/component world and simple simulation systems.

input/
  Typed input mapping system (device codes → semantic actions).

ui/
  Immediate-mode UI widgets, themes, and pointer interaction helpers.

runtime/
  Platform-agnostic Game contract, Context, Driver, input/control plumbing,
  frame production, and optional rewind history.

cmd/demos/football/raster/
  Offline render demo that generates a PNG and frame-by-frame PNG sequence.

cmd/demos/football/webCanvas/
  Browser-based canvas demo with live simulation and debug visualization.

cmd/benchsave/
  Cross-platform helper that runs Go benchmarks and saves a Markdown report.

docs/
  Additional notes and visual explanations.
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

## Running The Browser Demo

Start the WebSocket server:

```powershell
go run ./cmd/demos/football/webCanvas
```

Then open `http://localhost:8080` in your browser. Use WASD or arrow keys to move the controlled player.
The demo also includes a small screen-space UI overlay with a score panel and reset button.

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
