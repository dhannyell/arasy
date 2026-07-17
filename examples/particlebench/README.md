# WebGPU particle benchmark

Build and serve the standalone browser benchmark:

```powershell
$env:GOOS='js'; $env:GOARCH='wasm'
go build -o output/wasm/particlebench/app.wasm ./examples/particlebench/wasm
Remove-Item Env:GOOS; Remove-Item Env:GOARCH
go run ./cmd/frameforge dev -s output/wasm/particlebench -a 127.0.0.1:8091
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

Moving magenta tracers and amber sparks scale with the workload from the first
stage through the safety ceiling. They are a proportionally sampled visual aid,
not part of the workload target.
