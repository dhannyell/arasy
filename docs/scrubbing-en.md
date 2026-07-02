# Bidirectional Time Scrubbing

This guide explains the runtime's time scrubbing system: how to move backward
and forward through simulation time non-destructively, and how
checksum-verified determinism makes that possible.

## Core idea

The engine has two time-travel mechanisms, built on different foundations:

```text
rewind    (Back/BackToTick)  ->  snapshots     ->  bounded window, destructive
scrubbing (ScrubToTick)      ->  replays a log ->  full range, non-destructive
```

**Rewind** restores complete state snapshots (World, input, UI, game state)
captured every tick. The window is bounded by `Config.HistoryLen` and every
jump back consumes history — it is an undo.

**Scrubbing** never stores the future: it **reconstructs** any tick by
re-simulating from tick 0 with the complete input log. Because the simulation
is deterministic, the same log always produces exactly the same state. That
is why dragging the slider `100 -> 50 -> 100` lands bit-for-bit on the same
tick 100 you left.

## How it works

The `Driver` keeps an always-on log of every input event, stamped with the
tick it was applied on. `ScrubToTick(target)`:

1. re-runs the game on an internal driver from tick 0 up to the target,
   injecting the logged events on the exact ticks they originally happened
   (via `runtime.Replay`);
2. adopts the reconstructed state: World, input, UI, game state and the
   rewind snapshot window, all consistent with the target tick.

The reachable range is `[0, MaxRecordedTick]` (exposed via
`DriveState.MaxRecordedTick`), independent of `HistoryLen`. Scrubbing
backward never lowers that ceiling — it only moves when the timeline forks
(below).

```text
ControlScrubToTick{Tick: target}  ->  pause + re-simulate 0..target  ->  exact state
```

The web canvas wires its timeline slider to this command; any platform can do
the same through `Driver.Control`.

## Timeline forking

Rewinding and *watching* changes nothing. The timeline only diverges the
moment a **simulation step runs from a tick earlier than the ceiling** — from
that point on, what happens live is no longer the recorded future (recorded
events are not re-injected during live play). At that instant the Driver
forks:

- the recorded future past the fork point is discarded;
- the ceiling (`MaxRecordedTick`) drops to the fork point and grows again
  with live play;
- scrubbing after the fork reconstructs the **new** branch — the one the
  user actually watched — and the past before the fork stays intact.

Input alone never forks. Events arriving while the simulation is rewound and
paused (moving the mouse over the canvas, for example) are staged: they are
committed to the log if a step consumes their state (the fork), or discarded
if another scrub/back/reset rewinds first. This guarantees that "just
looking" never destroys the future.

## Checksum: the proof that scrubbing is exact

`World.Checksum()` is a deterministic hash of the simulation state, exposed
through `Driver.Checksum()`. Two states with the same checksum are the same
state — which is how the tests prove scrubbing is exact:

```text
original run:  checksum recorded every tick
scrub 15, 45, 5, 60, 30:  each jump's checksum == the original tick's checksum
```

The same mechanism verifies explicit recordings:

```go
driver.StartRecording()            // at tick 0
// ... play the game ...
rec := driver.StopRecording()

replay := runtime.Replay(&MyGame{}, log, rec)
replay.Checksum() == driver.Checksum() // bit-for-bit reproduction
```

`Replay` resets and re-runs the game instance it receives — pass a fresh
instance, not one attached to a live driver. Recordings must start at tick 0
(a fresh driver, or right after a reset); a reset in the middle of a
recording restarts the capture, because ticks restart from zero.

## The contract: your game must be deterministic

Scrubbing inherits the replay requirements. Inside `Update`:

- no global RNG (unseeded `math/rand`) — derive randomness from `ctx.Tick`;
- no wall-clock time, I/O, or state outside the World/`StateSlot`;
- a `StateSlot[T]` whose `T` has slice/map/pointer fields must implement
  `Clone() T` (see `runtime.Cloneable`).

If the game breaks the contract, scrubbing reconstructs a state different
from the one that was watched — the Driver's per-tick debug checksum log
exists precisely to expose that.

## Costs and limits

- **Cost per scrub**: O(target tick) — every jump re-simulates from tick 0.
  Cheap for dev sessions and demos; sparse keyframes are the natural
  evolution if long sessions make it hurt.
- **Memory**: the log stores only input events (tiny), never world
  snapshots. `ControlReset` clears the log and the ceiling.
- **Rewind still exists**: `Back`/`BackToTick` remain useful as a fast,
  short-range undo with no re-simulation cost; scrubbing covers the full
  range.
