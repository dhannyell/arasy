# WebGPU particle benchmark

Build and serve the standalone browser benchmark:

```powershell
go run ./cmd/arasy dev ./examples/bench/particlebench/wasm -a 127.0.0.1:8091
```

It starts with a target of 100 live particles, doubles that target every three
seconds while rolling FPS stays at or above 59 and frame-time P99 stays at or
below 20 ms, and only adds another 65,536-slot emitter when that target exceeds
the current pool. It stops after three consecutive failing reports, retaining
the last stable load, or at the 16.7-million-particle / approximately 1 GiB
particle-buffer safety ceiling. `RESTART` begins a fresh ramp.

The HUD reports rolling FPS, mean frame time, frame-time P99, and the most
recent browser event-to-render-submission latency. The last metric is not
display scan-out or hardware input-to-photon latency.

On adapters that grant WebGPU's `timestamp-query` feature, the HUD also
splits GPU frame cost into `emit` (the shared kick/emit compute pass), `step`
(the particle simulate pass) and `draw` (the main render pass). Browsers
quantize these timestamps (Chromium: 100 µs), and the sample lags its frame
by at least one frame because the readback is asynchronous.

Moving magenta tracers and amber sparks scale with the workload from the first
stage through the safety ceiling. They are a proportionally sampled visual aid,
not part of the workload target.
