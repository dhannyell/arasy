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

- [Runtime guide](docs/runtime.md)
- [UI guide](docs/ui.md)
- [Math visual guide](docs/math2d-visual-guide.md)
- [Camera, world, and viewport](docs/camera-world-viewport.md)
- [Architectural review (2026-06-26)](docs/revisao-arquitetural-2026-06-26.md)
- [Internal development notes](DEVELOPMENT.md)

## License

FrameForge is licensed under the [MIT License](LICENSE).
