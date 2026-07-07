# Runtime

This guide explains the role of the `runtime` package in FrameForge.

`runtime` is the layer that connects a game to a platform without making the
game know about Canvas, HTTP, PNG files, native windows, or any other edge
detail.

## Main Idea

```text
game code  ->  runtime  ->  scene/render/input
platform   ->  runtime
```

The game implements a small interface.

The platform creates a `runtime.Driver` and calls explicit methods:

```text
ApplyInput  ->  Advance  ->  Frame
Control     ->  Advance
```

`runtime` coordinates lifecycle, input, development controls, optional history,
and frame production.

It does not draw pixels and it does not open sockets.

## Where It Fits

Dependency rule:

```text
cmd/* -> platform/* -> runtime -> { scene, render, input } -> math2d
```

This means:

- `runtime` may import `scene`, `render`, and `input`.
- `platform/web/canvas` may import `runtime`.
- `platform/raster` may import `runtime`.
- `runtime` does not import any platform.
- `scene`, `render`, `math2d`, and `input` do not import `runtime`.

This separation lets the same game run on multiple outputs:

```text
cmd/demos/football/webCanvas  -> platform/web/canvas -> runtime
cmd/demos/football/raster     -> platform/raster     -> runtime
future native                 -> platform/native     -> runtime
future wasm/webgpu            -> platform/...        -> runtime
```

## The Game Contract

A game implements `runtime.Game`:

```go
type Game interface {
    Setup() Config
    Init(ctx *UpdateContext)
    Update(ctx *UpdateContext, dt float64)
    Render(ctx *RenderContext) []render.Command
}
```

### Setup

`Setup` returns portable configuration:

- base camera;
- clear color;
- simulation bounds;
- bounce configuration;
- physics configuration;
- simulation rate;
- target FPS;
- input bindings;
- rewind history size;
- optional UI theme.

Platform-specific configuration stays outside `Setup`.

Examples of configuration that does not belong in `Setup`:

- HTTP address;
- window title;
- PNG path;
- output image size;
- WebSocket options.

Those belong to each platform's `Options`.

### Init

`Init` populates a fresh `scene.World`.

It is called:

- at startup;
- after reset.

The game receives `ctx.World` directly. `runtime` does not hide the world behind
a larger wrapper.

### Update

`Update` runs one fixed simulation tick.

The game usually:

1. Reads `ctx.Input`.
2. Updates components in `ctx.World`.
3. Calls `ctx.HoldSimulation()` if physics should stay frozen for this tick.

After `Update`, `runtime` calls `World.Step(dt, ctx.StepConfig)`
automatically. Games that need to react to same-tick contacts implement
`runtime.PostStepper` and read `ctx.World.Contacts()` in `PostStep`.

That decision is important: tick semantics stay centralized in the Driver,
while the game can still hold simulation for individual ticks when needed.

### Render

`Render` returns drawing commands:

```go
func (g *Game) Render(ctx *runtime.RenderContext) []render.Command {
    return ctx.World.RenderCommands()
}
```

These commands are backend-agnostic.

`runtime` does not know whether they will become:

- pixels in an `image.RGBA`;
- JSON for Canvas;
- native draw calls;
- GPU commands in the future.

## Contexts

`runtime` separates update and render context.

```go
type UpdateContext struct {
    World      *scene.World
    Input      input.State
    Pointer    input.PointerFrame
    UI         ui.Interactor
    Log        *slog.Logger
    Tick       uint64
    Rand       *Rand
    StepConfig scene.StepConfig
}

type RenderContext struct {
    World *scene.World
    UI    ui.Drawer
    Debug scene.DebugLayers
    Time  float64
    Dt    float64
    Log   *slog.Logger
    Tick  uint64
}
```

Both contexts expose `Camera()`. The render context can also adjust the current
frame camera with `SetCamera(cam)`.

### Deterministic RNG

Use `ctx.Rand` inside `Init`, `Update`, or `PostStep` for simulation
randomness. The stream is seeded by `Config.Seed`, is part of the driver
snapshot, and is restored by rewind, scrub, and replay.

`RenderContext` does not expose an RNG. Render runs per frame, not per tick;
drawing random values during render would break determinism.

### World

`World` is the simple ECS state for the scene.

The game has direct control over it.

### Input

`Input` is the semantic state produced by `input.Mapper`.

The game reads actions:

```go
ctx.Input.Value(input.ActionMoveX)
ctx.Input.Pressed(input.ActionJump)
```

It does not need to know whether an action came from keyboard, gamepad, mouse,
or another future source.

### Log

`Log` is a logger already scoped for the game.

The engine core still does not depend on a global logger.

### Tick

`Tick` is the current simulation tick number.

It is useful for debugging, determinism, replay, and logs.

### Debug

`Debug` is a visual layer mask.

The platform may toggle overlays, but the game decides what to draw:

```go
if ctx.Debug.Has(scene.DebugColliders) {
    // append debug commands
}
```

### Camera

`Camera()` starts from the base camera in `Config`.

The game may adjust the camera in `Render`, for example for shake, zoom, or
visual panning:

```go
cam := ctx.Camera()
cam.World = shiftedWorld
ctx.SetCamera(cam)
```

`Driver.Frame(elapsed)` returns the resulting camera together with the commands.

### StepConfig

`StepConfig` combines simulation configuration from `Config`, such as bounds and
bounce and physics.

The Driver passes it to `World.Step` automatically after `Update`. The game may
adjust `ctx.StepConfig` during `Update` to affect only the current tick's step.

## Driver

`Driver` is the portable engine state machine.

It owns:

- game instance;
- portable configuration;
- current world;
- input mapper;
- input buffer;
- development controls;
- current tick;
- logger;
- optional rewind history.

Each platform creates a `Driver`:

```go
driver := runtime.NewDriver(game, log)
cfg := driver.Config()
```

Then it drives the loop.

## Input Flow

The platform translates native events into `input.Event`.

Conceptual example:

```text
browser keydown "KeyW"
  -> input.Event{Source: keyboard KeyW, Kind: ButtonDown}
  -> driver.ApplyInput(events)
  -> input.Mapper
  -> ctx.Input in Update
```

The game does not read raw events.

It reads semantic state:

```go
ctx.Input.Value(input.ActionMoveY)
```

That keeps the same game reusable across platforms.

## Simulation Flow

A normal tick:

```text
platform drains input
platform calls driver.ApplyInput(events)
platform calls driver.Advance(dt)

driver:
  captures a snapshot when HistoryLen > 0
  computes input.State
  builds UpdateContext
  calls game.Update(ctx, dt)
  calls World.Step(dt, ctx.StepConfig), unless Update held simulation
  calls game.PostStep(ctx, stats), when implemented
  increments Tick
  ends the input frame
```

If the game is paused, `Advance` respects the development controls.

## Render Flow

A render frame:

```text
platform calls driver.Frame(elapsed)

driver:
  accumulates renderTime += elapsed; renderDt = elapsed
  builds RenderContext (ctx.Time/Dt reflect render-time, not sim-time)
  calls game.Render(ctx)
  returns Frame{Camera, Commands}

platform:
  converts Commands to its output
```

`elapsed` is the real time since the previous render frame, measured by the host.
While the simulation is paused or running at a different speed, `ctx.Time`/`ctx.Dt`
keep advancing at render-rate — correct for cosmetic UI animations.
`Commands()` is a helper that calls `Frame(0)` without advancing the render clock.

On canvas, the platform serializes commands to JSON.

On raster, the platform draws commands into an `image.RGBA`.

## Development Controls

`runtime` also centralizes common development controls:

- play;
- pause;
- manual step;
- reset;
- speed;
- back one snapshot;
- back to a tick;
- back a few seconds;
- select debug layers.

The platform only translates user intent into `ControlCommand`:

```go
driver.Control(runtime.ControlCommand{
    Kind: runtime.ControlPause,
})
```

A future native platform can reuse the same controls.

## History And Rewind

If `Config.HistoryLen > 0`, the `Driver` captures snapshots before ticks.

A snapshot stores the state needed to restore the engine timeline, including the
world, input state, UI interaction state, pointer state, game state, and tick.

This allows time travel without making the platform know ECS details.

History is a development/debugging tool. It should not become a game rule.

## Concurrency

`Driver` is not thread-safe by itself.

That is deliberate.

The platform decides how to serialize access.

In the current canvas platform, for example:

- `Advance` runs with an exclusive lock;
- `Frame` runs with a read lock;
- input and control commands enter through queues.

Keeping `Driver` free of internal locks makes the package simpler and easier to
test.

## What Does Not Belong In Runtime

Do not put these in `runtime`:

- `net/http`;
- WebSocket;
- DOM;
- Canvas;
- PNG/GIF paths;
- native window APIs;
- GPU details;
- football rules;
- pixel rendering.

Those belong in:

- `platform/*`, when they are input/output edge details;
- `cmd/*`, when they are executable configuration;
- game/app code, when they are product rules.

## Summary

```text
runtime.Game:
  portable game contract

runtime.UpdateContext / runtime.RenderContext:
  data delivered to the game during ticks and frames

runtime.Driver:
  portable state machine driven by platforms

ApplyInput:
  raw platform events -> semantic input

Advance:
  runs simulation ticks

Frame:
  produces camera + render commands

Control:
  play/pause/step/reset/rewind/debug
```

`runtime` exists so the game can be written once and run on multiple platforms
with minimal adaptation.
