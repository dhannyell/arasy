# FrameForge

FrameForge is an experimental 2D engine written in Go.

The project is focused on learning, clarity, deterministic simulation, and low coupling between core engine concepts and rendering backends. It currently provides a small foundation for 2D math, fixed timestep loops, render commands, a raster renderer, and a lightweight scene/entity model.

FrameForge is not trying to compete with commercial engines. The goal is to build a compact, inspectable engine from first principles using mostly the Go standard library.

## Status

FrameForge is in early development.

Current capabilities:

- 2D vector and rectangle math.
- Fixed timestep helper.
- Camera with aspect-ratio modes.
- Backend-agnostic render commands.
- Raster renderer using Go's standard `image` packages.
- Filled/stroked rectangles.
- Filled/stroked circles.
- Lines, arcs, and point markers.
- Basic scene world with entities and components.
- Transform, circle sprite, and velocity components.
- Simple integration system for linear movement.
- PNG and GIF render demo.

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

scene/
  Minimal entity/component world and simple simulation systems.

cmd/rasterdemo/
  Small offline render demo that generates PNG and GIF output.

cmd/webcanvademo/
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

## Running The Demo

```powershell
go run ./cmd/rasterdemo
```

The demo writes files to:

```text
output/renderdemo.png
output/renderdemo.gif
```

The PNG is a static high-resolution render. The GIF is a smaller animated preview used to validate movement and rendering.

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

- [Math visual guide](docs/math2d-visual-guide.md)
- [Camera, world, and viewport](docs/camera-world-viewport.md)
- [Internal development notes](DEVELOPMENT.md)

## License

FrameForge is licensed under the [MIT License](LICENSE).
