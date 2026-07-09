# Audio

This guide describes FrameForge's audio model: games declare clips, emit
deterministic intents through `ctx.Audio`, and hosts consume output from the
`runtime.Driver`.

## Model

Audio has two forms:

```text
cue    -> one-shot sound, fire-and-forget, stamped with tick and seq
track  -> desired continuous sound, reconciled by the host
```

Use cues for effects such as impacts, UI confirmations, or errors. Use tracks
for music, ambience, or long loops. The runtime never decodes or plays sound;
it only carries deterministic intents to the host.

## Game API

`runtime.UpdateContext` exposes `ctx.Audio` in `Init`, `Update`, and
`PostStep`. Clips are declared in the same `manifest.json` used by textures
and sprites.

```json
{
  "version": 1,
  "audio": [
    { "id": "hit", "file": "hit.wav" },
    { "id": "music", "file": "music.wav" }
  ]
}
```

```go
func (g *Game) Init(ctx *runtime.UpdateContext) {
	ctx.Audio.SetTrack("music", audio.Track{ClipID: "music", Loop: true, Gain: 0.4})
}

func (g *Game) PostStep(ctx *runtime.UpdateContext, stats scene.StepStats) {
	ctx.Audio.PlayWith("hit", audio.CueOptions{Gain: 0.8})
}
```

`Gain` is linear. Zero means 1.0, keeping zero-value structs useful. `Pan` is
in `[-1, 1]`.

## Host Contract

Hosts call `Driver.ConsumeAudio()` after advancing simulation, from the same
serialized context that calls `AdvanceTime` or `Step`.

```text
ApplyInput -> AdvanceTime/Step -> ConsumeAudio -> deliver to audio backend
```

`ConsumeAudio` drains pending cues and returns a copy of desired tracks. Cues
accumulate until consumed, with a bounded queue. Tracks are continuous state:
the host compares desired tracks with what is playing, starts missing tracks,
stops stale tracks, and adjusts gain when needed.

The web canvas serves the clips declared by `Config.Assets.AudioClips()` under
`/audio/<path>` when given an `AudioFS`, sends a manifest to new clients, and
transports audio messages over the WebSocket from the simulation loop.

## Rewind, Scrub, and Replay

Audio follows the simulation timeline, but it is not part of
`World.Checksum()`.

| Operation | Cues | Tracks |
|---|---|---|
| Normal play | emitted and drained once | reconciled by the host |
| Rewind | pending future cues are discarded | restored by snapshot |
| Scrub | re-simulated cues are suppressed | reconstructed for target tick |
| Fork after scrub | cues emit again on the new timeline | follow new state |
| Explicit replay | does not change checksum | state is reconstructed |

Emit audio only from `Init`, `Update`, or `PostStep`. Never emit from
`Render`: frames can run at a different cadence from ticks and would break
determinism.
