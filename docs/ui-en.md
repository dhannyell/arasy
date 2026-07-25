# UI

This guide explains Arasy's `ui` package.

The package implements a small immediate-mode UI intended for HUDs, simple
menus, debug overlays, and lightweight tools inside the engine.

## Main Idea

```text
Update:
  pointer frame -> ctx.UI.In(space).Clicked(...) -> changes deterministic state

Render:
  world + current cursor -> ctx.UI.In(space).Button(...) -> render.Command
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

`ui/space.go` defines the `ui.Space` parameter. `ctx.UI.In(space)` returns a
lightweight scope with every widget exposed exactly once — the space is
stated once per block, and the returned scope holds no state beyond
`(ui, space)`, so there is nothing to restore afterward.

```go
d := ctx.UI.In(ui.SpacePlayfield)
d.Button("btn_reset", "Reset", reset)

in := ctx.UI.In(ui.SpacePlayfield)
if in.Clicked("btn_reset", reset) {
    g.reset()
}
```

Available spaces:

- `ui.SpaceRef`: uses the UI reference resolution (`Config.Camera.Viewport`) and
  applies `AspectFit` inside the physical viewport. Use it for menus,
  generic HUDs, and panels that should scale responsively.
- `ui.SpacePlayfield`: anchors UI inside the rectangle where `World` was
  actually drawn after the camera aspect strategy. Use it for elements that must
  not leave game borders, such as score bars, minimaps, or field-bound
  indicators.
- `ui.SpaceScreen`: uses physical viewport pixels, with no scaling and no
  letterboxing. Use it for pixel-exact overlays, such as debug HUDs or icons
  pinned to a window corner.

For interaction to line up correctly, use the same `Space` in `Update` and
`Render` for the same widget.

## Update vs Render

Interaction that changes simulation must happen in `Update`.

```go
func (g *Game) Update(ctx *runtime.UpdateContext, dt float64) {
    _, reset := hudLayout(ctx.Camera().Viewport)
    if ctx.UI.In(ui.SpaceRef).Clicked("btn_reset", reset) {
        g.resetMatch(ctx.World)
    }
}
```

Drawing must happen in `Render`.

```go
func (g *Game) Render(ctx *runtime.RenderContext) []render.Command {
    score, reset := hudLayout(ctx.Camera().Viewport)

    d := ctx.UI.In(ui.SpaceRef)
    d.Panel(score)
    d.Label("0 - 0", score.Center())
    d.Button("btn_reset", "Reset", reset)

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
- wheel accumulated during the tick, in line units;
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

`ctx.UI` only exposes `In(space)` plus a handful of space-free methods
(`RowHeight`, `HeaderHeight`, `TableBodyRect`, `SetFont`, animation, and
groups). Everything that depends on a rectangle/position lives on the scope
returned by `In`.

`DrawScope` (Render, via `ctx.UI.In(space)`):

- `Button(id, label, rect)`
- `Card(rect, accent)`
- `Badge(text, center, bg)`
- `Label(text, pos)`
- `Panel(rect)`
- `ProgressBar(rect, value)`
- `Slider(id, rect, value, opts)`
- `Table(id, rect, cols, rowCount, selected, cell)`
- `TableStyled(id, rect, cols, rowCount, selected, cell, style)`
- `List(id, rect, rowH, rowCount, selected, draw)`
- `ScrollArea(id, view, contentH)`
- `Hovered(rect)`
- `FillRect(rect, fill)`
- `DrawText(text, pos, color, size, align, baseline)`
- `DrawTextRot(text, pos, color, size, rotation, align, baseline)`
- `DrawCircle(center, radius, fill, stroke, strokeWidth)`
- `DrawLine(from, to, width, color)`
- `DrawPoly(fill, stroke, strokeWidth, points...)`
- `FX()`

`InteractScope` (Update, via `ctx.UI.In(space)`):

- `Clicked(id, rect)`
- `Activated(id, rect)`
- `Focusable(id, rect)`
- `Hovered(rect)`
- `ScrollOffset(id, view, contentH)`
- `SliderInput(id, rect, value, opts)`
- `TableHit(id, rect, cols, rowCount, selected)`
- `ListHit(id, rect, rowH, rowCount)`

`Button`, `Panel`, and `ProgressBar` draw rectangles. `Label` and button text use
`render.DrawText` with centered alignment and middle baseline.

`ProgressBar` clamps `value` to `[0, 1]`.

`Slider` follows the UI's deterministic two-phase split. The game consumes
pointer input during `Update` with `SliderInput`, then draws the same ID,
rectangle, and options during `Render` with `Slider`:

```go
var volumeSlider = ui.SliderOptions{Min: 0, Max: 1, Step: 0.05}

func (g *Game) Update(ctx *runtime.UpdateContext, dt float64) {
    if value, changed := ctx.UI.In(ui.SpaceRef).SliderInput("volume", volumeRect, g.Volume, volumeSlider); changed {
        g.Volume = value
    }
}

func (g *Game) Render(ctx *runtime.RenderContext) []render.Command {
    ctx.UI.In(ui.SpaceRef).Slider("volume", volumeRect, g.Volume, volumeSlider)
    return nil
}
```

The value belongs to game state; UI state persists only pointer capture and
focus. `Step <= 0` keeps the value continuous. A positive `Step` snaps to
increments measured from `Min`, while keeping `Max` reachable as the final snap
point. Clicking the track jumps the thumb; dragging
remains captured outside the rectangle and clamps to `[Min, Max]`.

`Card` draws a rounded panel with an optional accent bar. `Badge` draws a small
chip sized by `render.MeasureText`.

## Scroll, Tables, And Focus

Scroll and tables follow the immediate-mode pattern in two phases:

```go
func (g *Game) Update(ctx *runtime.UpdateContext, dt float64) {
    in := ctx.UI.In(ui.SpaceRef)
    if row := in.TableHit("players", tableRect, cols, len(players), selected); row >= 0 {
        selected = row
    }
    in.ScrollOffset("players", bodyRect, float64(len(players))*32)
}

func (g *Game) Render(ctx *runtime.RenderContext) []render.Command {
    ctx.UI.In(ui.SpaceRef).Table("players", tableRect, cols, len(players), selected, func(row, col int) string {
        return players[row].Cell(col)
    })
    return nil
}
```

`ScrollOffset` belongs in `Update` because it consumes `PointerFrame.Wheel` and
stores the offset in `ui.State`. `ScrollArea` and `Table` draw the scrollbar
and apply `render.BaseCommand.Clip` to body commands. The offset is part of the
runtime snapshot, so rewind/scrub restores list position.

`TableColumn.Width == 0` means flex column: remaining width is split across flex
columns. Use `AlignRight` for numbers; text width comes from the canonical
`render.MeasureText` metric.

Keyboard focus is intentionally small: `Focusable` registers widgets in call
order, Tab advances focus, Enter activates the focused widget, and Esc clears
focus. `Activated` is the recommended click + Enter superset; `Clicked` remains
available when a widget should respond only to pointer input.

## UI Animation

UI animation is cosmetic: it runs in `Render`, uses the
`Driver.Frame(elapsed)` clock, and is not stored in rewind snapshots. Simulation
pause, speed, and rewind do not rewind these effects; only a trigger derived
from deterministic game state can restart them.

The main API is organized in layers:

- `ctx.UI.In(space).FX()` — ready-made recipes for common cases.
- `ctx.UI.Effect(id, opts)` — a composable timeline for HUDs and custom events.
- primitives (`Approach`, `Spring`, `Tween`, `Clock`, `Reset`, `SequenceKeyed`,
  `Track`) — the escape hatch for fine control.

`ScreenTransition` complements these layers for whole-screen changes: it uses
`BeginGroup`/`EndGroup` underneath to move or fade every command emitted by a
screen.

### `FX`: Ready-Made Recipes

Use `FX` when an effect already exists as a recipe and you want little
configuration. `FX` is obtained from a `DrawScope`, so it already carries the
coordinate space. `Toast` draws a panel + text in that space:

```go
ctx.UI.In(ui.SpaceRef).FX().Toast("save_notice", "Saved", rect, ui.ToastOptions{Key: saveKey})

ctx.UI.In(ui.SpaceScreen).FX().Toast("pause", "Paused", rect, ui.ToastOptions{Key: pauseKey})
```

`ToastOptions.Key` follows `SequenceKeyed` semantics: `0` means inactive and
clears the retained trigger; a new non-zero value restarts the animation. Use a
tick or monotonically increasing counter from game state to trigger the effect.

`ToastOptions` can also tune duration, layer, and appearance through
`ToastStyle`. The motion recipe is fixed by design; if you need to change enter
motion, fade, or pulse, prefer `Effect`.

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

### Screen Transitions

Use `ScreenTransition` when deterministic game state selects the active UI
screen and you want to animate the visual change. The key should identify the
current screen: `0` clears retained state; the first non-zero key appears
without animating; a key change starts the transition.

```go
tr := ctx.UI.ScreenTransition("main_screen", screenKey, ui.TransitionOptions{
    Kind:     ui.TransitionSlideLeft,
    Duration: 0.32,
    Ease:     ease.OutCubic,
})

if outgoing, ok := tr.Outgoing(); ok {
    ctx.UI.BeginGroup(tr.OutgoingGroup())
    drawScreen(ctx.UI, outgoing)
    ctx.UI.EndGroup()
}

ctx.UI.BeginGroup(tr.IncomingGroup())
drawScreen(ctx.UI, tr.Incoming())
ctx.UI.EndGroup()
```

The built-in recipes are `TransitionFade`, `TransitionSlideLeft`, and
`TransitionSlideRight`. They are render-only: they use the `AnimStore` and the
render clock, do not enter the checksum, and do not alter simulation.

`BeginGroup`/`EndGroup` can also be used directly. Everything drawn between
them receives a `GroupOptions` transform with `Offset`, `Fade`, and `Space`.
The zero value is a no-op: no displacement and fully opaque.

```go
ctx.UI.BeginGroup(ui.GroupOptions{
    Offset: math2d.Vec2{X: 24},
    Fade:   0.25,
    Space:  ui.SpaceRef,
})
d := ctx.UI.In(ui.SpaceRef)
d.Panel(panel)
d.Label("Settings", panel.Center())
ctx.UI.EndGroup()
```

Input is not automatically suppressed during a transition. `Clicked` still runs
in `Update`, while the transition runs in `Render`; if a screen needs to block
clicks for a few ticks after navigation, store that lockout in game state.

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
effects matures, prefer creating a new `FX` method with minimal options instead
of adding more loose flags:

```go
ctx.UI.In(ui.SpaceRef).FX().Toast("goal", "GOL!", rect, ui.ToastOptions{
    Key: goalTick,
    Style: ui.ToastStyle{TextSize: 24},
})
```

`FX` should be quick and opinionated. `Effect` should be composable, including
custom motion. The primitives should provide fine control without forcing new
recipes.

## IDs

Interactive widgets use stable IDs:

```go
ctx.UI.In(ui.SpaceRef).Button("btn_reset", "Reset", reset)
if ctx.UI.In(ui.SpaceRef).Clicked("btn_reset", reset) {
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
- progress bars;
- sliders;
- cards, rows, headers, badges, text, and focus.

Partial themes are completed by `ui.WithDefaults`: zero-value slots inherit
from `DefaultTheme()`. This lets older games customize only `Button` or `Panel`
without filling the new slots used by scroll, tables, and focus.

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
    if ctx.UI.In(ui.SpaceRef).Clicked("btn_reset", reset) {
        g.reset()
    }
}

func (g *Game) Render(ctx *runtime.RenderContext) []render.Command {
    score, reset := hudLayout(ctx.Camera().Viewport)

    d := ctx.UI.In(ui.SpaceRef)
    d.Panel(score)
    d.Label("0 - 0", score.Center())
    d.Button("btn_reset", "Reset", reset)

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
ctx.UI.In(space):
  immediate scope used by the game, one per coordinate space

Clicked:
  deterministic query called in Update

Button/Label/Panel/ProgressBar/Slider:
  widgets called in Render

Theme:
  visual widget style

LayerBase:
  layer range reserved above the game
```

The package exists to enable simple UI without coupling games to platforms and
without breaking the deterministic simulation model.
