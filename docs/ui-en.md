# UI

This guide explains FrameForge's `ui` package.

The package implements a small immediate-mode UI intended for HUDs, simple
menus, debug overlays, and lightweight tools inside the engine.

## Main Idea

```text
Update:
  pointer frame -> ctx.UI.Clicked(...) -> changes deterministic state

Render:
  world + current cursor -> ctx.UI.Button(...) -> render.Command
```

The UI does not talk to Canvas, DOM, native windows, or the system mouse
directly. It receives data already translated by `runtime` and produces commands
from the `render` package.

## Where It Fits

Dependency rule:

```text
cmd/* -> platform/* -> runtime -> ui -> { render, math2d, input }
```

This means:

- games use UI through `ctx.UI`;
- platforms do not draw UI directly;
- `ui` does not import `runtime`;
- `ui` does not know any platform;
- widgets generate `render.Command`, like the rest of the engine.

## Screen Space

Widgets receive rectangles and positions in a UI reference area:

```go
reset := math2d.NewRect(
    math2d.Vec2{X: ctx.Camera().Viewport.Width() - 100, Y: 8},
    math2d.Vec2{X: 92, Y: 28},
)
```

The second argument of `math2d.NewRect` is size, not the maximum corner.

By default, that reference area is `Config.Camera.Viewport`. This lets you draw
UI at a fixed design resolution and let `runtime` scale and center widgets when
the real viewport changes.

Internally, the package converts UI values to world space using the context's
camera:

```text
UI rect -> responsive UI camera -> camera.ScreenToWorld -> render.DrawRect
UI text size -> responsive UI camera -> camera.ScreenScalarToWorld -> render.DrawText
```

This keeps the render pipeline unified: both game objects and UI produce
backend-agnostic commands.

## Responsive UI With `ui.Space`

`ui/space.go` defines the `ui.Space` parameter used by the `*In(...)` widget
variants and by `ClickedIn(...)`.

```go
ctx.UI.ButtonIn("btn_reset", "Reset", reset, ui.SpacePlayfield)
if ctx.UI.ClickedIn("btn_reset", reset, ui.SpacePlayfield) {
    g.reset()
}
```

Available spaces:

- `ui.SpaceRef`: uses the UI reference resolution (`Config.Camera.Viewport`) and
  applies `AspectFit` inside the physical viewport. This is the default used by
  `Button`, `Panel`, `Label`, `ProgressBar`, and `Clicked`. Use it for menus,
  generic HUDs, and panels that should scale responsively.
- `ui.SpacePlayfield`: anchors UI inside the rectangle where `World` was
  actually drawn after the camera aspect strategy. Use it for elements that must
  not leave game borders, such as score bars, minimaps, or field-bound
  indicators.
- `ui.SpaceScreen`: uses physical viewport pixels, with no scaling and no
  letterboxing. Use it for pixel-exact overlays, such as debug HUDs or icons
  pinned to a window corner.

Methods without the suffix are shortcuts for `SpaceRef`:

```go
ctx.UI.Button("btn_play", "Play", play)
// equivalent to:
ctx.UI.ButtonIn("btn_play", "Play", play, ui.SpaceRef)
```

For interaction to line up correctly, use the same `Space` in `Update` and
`Render` for the same widget.

## Update vs Render

Interaction that changes simulation must happen in `Update`.

```go
func (g *Game) Update(ctx *runtime.UpdateContext, dt float64) {
    _, reset := hudLayout(ctx.Camera().Viewport)
    if ctx.UI.Clicked("btn_reset", reset) {
        g.resetMatch(ctx.World)
    }
}
```

Drawing must happen in `Render`.

```go
func (g *Game) Render(ctx *runtime.RenderContext) []render.Command {
    score, reset := hudLayout(ctx.Camera().Viewport)

    ctx.UI.Panel(score)
    ctx.UI.Label("0 - 0", score.Center())
    ctx.UI.Button("btn_reset", "Reset", reset)

    return ctx.World.RenderCommands()
}
```

This separation is deliberate:

- `Update` consumes tick-stamped input;
- `Render` may use hover/cursor data as transient visual state;
- gameplay remains deterministic;
- rewind can restore `activeID` and pointer state.

## Pointer

The runtime delivers an `input.PointerFrame` to UI during Update.

The frame contains:

- final pointer position for the tick;
- whether the cursor is inside the surface;
- final button states;
- button transitions with positions.

Transitions preserve important cases:

```text
down outside -> up inside  = no click
down inside  -> up outside = no click
down inside  -> up inside  = click
leave while button is down = synthetic release
```

Persistent UI state, such as `activeID`, lives in `runtime.Driver` and is stored
in recorder snapshots.

## Widgets

Available widgets:

- `Button(id, label, rect)`
- `ButtonIn(id, label, rect, space)`
- `Label(text, pos)`
- `LabelIn(text, pos, space)`
- `Panel(rect)`
- `PanelIn(rect, space)`
- `ProgressBar(rect, value)`
- `ProgressBarIn(rect, value, space)`
- `Clicked(id, rect)`
- `ClickedIn(id, rect, space)`

`Button`, `Panel`, and `ProgressBar` draw rectangles. `Label` and button text use
`render.DrawText` with centered alignment and middle baseline.

`ProgressBar` clamps `value` to `[0, 1]`.

## UI Animation

UI animation is cosmetic: it runs in `Render`, uses the
`Driver.Frame(elapsed)` clock, and is not stored in rewind snapshots. Simulation
pause, speed, and rewind do not rewind these effects; only a trigger derived
from deterministic game state can restart them.

The API has three layers:

- `ctx.UI.FX()` — ready-made recipes for common cases.
- `ctx.UI.Effect(id, opts)` — a composable timeline for HUDs and custom events.
- primitives (`Approach`, `Spring`, `Tween`, `Clock`, `Reset`, `SequenceKeyed`,
  `Track`) — the escape hatch for fine control.

### `FX`: Ready-Made Recipes

Use `FX` when an effect already exists as a recipe and you want little
configuration. `Toast` draws a panel + text in UI coordinates:

```go
ctx.UI.FX().Toast("save_notice", "Saved", rect, ui.ToastOptions{Key: saveKey})
```

`ToastIn` accepts an explicit `ui.Space`:

```go
ctx.UI.FX().ToastIn("pause", "Paused", rect, ui.SpaceScreen, ui.ToastOptions{Key: pauseKey})
```

`ToastOptions.Key` follows `SequenceKeyed` semantics: `0` means inactive and
clears the retained trigger; a new non-zero value restarts the animation. Use a
tick or monotonically increasing counter from game state to trigger the effect.

`ToastOptions` can also tune duration, layer, colors, text, enter position, fade,
and pulse. Keep these adjustments small. If the recipe starts needing many
custom fields, prefer `Effect`.

### `Effect`: Composition Without Boilerplate

Use `Effect` when the animation is game-specific but still fits a simple
timeline. It wraps `SequenceKeyed` and names common transforms:

```go
fx := ctx.UI.Effect("goal_banner", ui.EffectOptions{Key: lastGoalTick, Duration: 2})
if !fx.Done() {
    rect := fx.SlideY(finalRect, -60, 12, 0, 0.35, ease.OutBack)
    alpha := fx.FadeOut(1.6, 2, ease.InQuad)
    pulse := fx.Pulse(0.35, 1.6, 0.05, 8)

    // Build game or UI commands using rect/alpha/pulse.
    _ = rect
    _ = alpha
    _ = pulse
}
```

`Effect` is the best choice for world-space effects: compute values with
`Effect` and emit `render.Command` values in the game's `Render`, using the game
camera.

### Primitives: Fine Control

Use primitives when you need direct control over numeric state:

- `Approach`: value exponentially follows a target, independent of FPS.
- `Spring`: damped spring motion.
- `Tween`/`Clock`/`Reset`: time-based progress.
- `SequenceKeyed`/`Track`: multi-step sequences.

Internal widgets also use these primitives. For example, `Button` animates
hover/press through `Approach` and `ButtonTheme.HoverSpeed`/`PressSpeed`.

### Evolution Direction

New recipes should avoid growing `ToastOptions` indefinitely. When a family of
effects matures, prefer extracting reusable presets and styles instead of adding
more loose flags:

```go
ctx.UI.FX().Toast("goal", "GOL!", rect, ui.ToastOptions{
    Key: goalTick,
    // Desired future: Style: ui.ToastSuccessStyle, Motion: ui.MotionBanner.
})
```

`FX` should be quick and opinionated. `Effect` should be composable. The
primitives should provide fine control without forcing new recipes.

## IDs

Interactive widgets use stable IDs:

```go
ctx.UI.Button("btn_reset", "Reset", reset)
if ctx.UI.Clicked("btn_reset", reset) {
    // ...
}
```

The ID must be the same in Update and Render. Duplicate IDs share interaction
state and should be avoided.

## Layers

`ui.LayerBase` reserves the UI layer range:

```go
const LayerBase = 10_000
```

Widgets increment layers from that value so they appear above normal game
commands.

## Theme

`runtime.Config` may receive a `UITheme`.

If the theme is the zero value, the driver uses `ui.DefaultTheme()`.

```go
func (g *Game) Setup() runtime.Config {
    return runtime.Config{
        UITheme: ui.DefaultTheme(),
    }
}
```

The theme defines styles for:

- normal, hovered, and active buttons;
- labels;
- panels;
- progress bars.

## Web Canvas Platform

On web canvas, the browser sends mouse events in backing pixels:

```text
mousemove  -> EventPointerMove
mousedown  -> EventButtonDown
mouseup    -> EventButtonUp
mouseleave -> EventPointerLeave
```

The server translates those events to `input.Event`, `runtime.Driver`
accumulates a `PointerFrame`, and the game consumes `ctx.UI` without knowing
about WebSocket or DOM.

## Complete Example

```go
func hudLayout(vp math2d.Rect) (score, reset math2d.Rect) {
    score = math2d.NewRect(
        math2d.Vec2{X: vp.Width()/2 - 60, Y: 8},
        math2d.Vec2{X: 120, Y: 28},
    )
    reset = math2d.NewRect(
        math2d.Vec2{X: vp.Width() - 100, Y: 8},
        math2d.Vec2{X: 92, Y: 28},
    )
    return
}

func (g *Game) Update(ctx *runtime.UpdateContext, dt float64) {
    _, reset := hudLayout(ctx.Camera().Viewport)
    if ctx.UI.Clicked("btn_reset", reset) {
        g.reset()
    }
}

func (g *Game) Render(ctx *runtime.RenderContext) []render.Command {
    score, reset := hudLayout(ctx.Camera().Viewport)

    ctx.UI.Panel(score)
    ctx.UI.Label("0 - 0", score.Center())
    ctx.UI.Button("btn_reset", "Reset", reset)

    return ctx.World.RenderCommands()
}
```

## What Does Not Belong In UI

Do not put these in `ui`:

- game rules;
- match state;
- WebSocket;
- DOM;
- Canvas;
- pixel rendering;
- a complex layout engine without real need.

Game-specific state should stay in the game. If it must survive rewind, it
should be captured by runtime as game state in its own layer, not as a
game-specific component inside `ui`.

## Summary

```text
ui.UI:
  immediate interface used by the game

Clicked:
  deterministic query called in Update

Button/Label/Panel/ProgressBar:
  widgets called in Render

Theme:
  visual widget style

LayerBase:
  layer range reserved above the game
```

The package exists to enable simple UI without coupling games to platforms and
without breaking the deterministic simulation model.
