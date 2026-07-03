# Benchmark Comparison Report

Generated at 2026-07-03T14:09:43-03:00 from every report family in `benchmarks`.

## bench-runtime

Only one report so far (`bench-runtime-2026-07-03-135320.md`) — nothing to compare yet, this is the baseline.

## bench-runtime-seek

Only one report so far (`bench-runtime-seek-2026-07-02.md`) — nothing to compare yet, this is the baseline.

## bench-runtime-seek-back

Only one report so far (`bench-runtime-seek-back-2026-07-02.md`) — nothing to compare yet, this is the baseline.

## bench-scene

_bench-scene-2026-07-03-125432.md → bench-scene-2026-07-03-134750.md_

Negative % = faster/smaller (improvement). Positive % = slower/bigger (regression).

| Benchmark | ns/op (old → new) | Δ ns/op | B/op (old → new) | Δ B/op | allocs/op (old → new) | Δ allocs/op |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| `BenchmarkDetectCircleOverlaps/narrow/10/spread-16` | 2256.6 → 2264.2 | +0.3% | 1248 → 1248 | +0.0% | 5 → 5 | +0.0% |
| `BenchmarkDetectCircleOverlaps/narrow/25/spread-16` | 14254.2 → 13831 | -3.0% | 7520 → 7520 | +0.0% | 6 → 6 | +0.0% |
| `BenchmarkDetectCircleOverlaps/narrow/50/spread-16` | 55948 → 55034.6 | -1.6% | 24432 → 24432 | +0.0% | 7 → 7 | +0.0% |
| `BenchmarkDetectCircleOverlaps/narrow/75/spread-16` | 130551.2 → 126935 | -2.8% | 79680 → 79680 | +0.0% | 9 → 9 | +0.0% |
| `BenchmarkDetectCircleOverlaps/narrow/100/spread-16` | 236712.6 → 225821.6 | -4.6% | 127264 → 127264 | +0.0% | 10 → 10 | +0.0% |
| `BenchmarkDetectCircleOverlaps/narrow/1000/spread-16` | 26391754.2 → 25606657 | -3.0% | 22784065 → 22784066 | +0.0% | 26 → 26 | +0.0% |
| `BenchmarkDetectCircleOverlaps/narrow/5000/spread-16` | 767393260 → 752687560 | -1.9% | 615133273.6 → 615133363.2 | +0.0% | 36 → 37 | +1.6% |
| `BenchmarkDetectCircleOverlapsGrid/grid/10/spread-16` | 2659.8 → 2424.6 | -8.8% | 1592 → 1592 | +0.0% | 27 → 27 | +0.0% |
| `BenchmarkDetectCircleOverlapsGrid/grid/25/spread-16` | 6579.2 → 6489 | -1.4% | 4424 → 4424 | +0.0% | 59 → 59 | +0.0% |
| `BenchmarkDetectCircleOverlapsGrid/grid/50/spread-16` | 19649.4 → 19948.8 | +1.5% | 16168 → 16168 | +0.0% | 143 → 143 | +0.0% |
| `BenchmarkDetectCircleOverlapsGrid/grid/75/spread-16` | 25376.8 → 30022.8 | +18.3% | 17896 → 17896 | +0.0% | 185 → 185 | +0.0% |
| `BenchmarkDetectCircleOverlapsGrid/grid/100/spread-16` | 30041.6 → 34601.4 | +15.2% | 19112 → 19112 | +0.0% | 213 → 213 | +0.0% |
| `BenchmarkDetectCircleOverlapsGrid/grid/1000/spread-16` | 497523.2 → 599900 | +20.6% | 296905.4 → 296905 | -0.0% | 2562 → 2562 | +0.0% |
| `BenchmarkDetectCircleOverlapsGrid/grid/5000/spread-16` | 2939289.4 → 3690380.4 | +25.6% | 1266605.4 → 1266559.4 | -0.0% | 8226 → 8226 | +0.0% |
| `BenchmarkDetectCircleOverlapsGridDense/grid/10/dense-16` | 2701 → 3453.8 | +27.9% | 1592 → 1592 | +0.0% | 27 → 27 | +0.0% |
| `BenchmarkDetectCircleOverlapsGridDense/grid/25/dense-16` | 10292.6 → 11554 | +12.3% | 4592 → 4592 | +0.0% | 51 → 51 | +0.0% |
| `BenchmarkDetectCircleOverlapsGridDense/grid/50/dense-16` | 18049.6 → 19919.4 | +10.4% | 6368 → 6368 | +0.0% | 55 → 55 | +0.0% |
| `BenchmarkDetectCircleOverlapsGridDense/grid/75/dense-16` | 57552.4 → 62570.6 | +8.7% | 20696 → 20696 | +0.0% | 84 → 84 | +0.0% |
| `BenchmarkDetectCircleOverlapsGridDense/grid/100/dense-16` | 112234.8 → 121704.8 | +8.4% | 38528 → 38528 | +0.0% | 105 → 105 | +0.0% |
| `BenchmarkDetectCircleOverlapsGridDense/grid/1000/dense-16` | 9749999.8 → 10836998.4 | +11.1% | 2123271.2 → 2123278 | +0.0% | 434 → 434 | +0.0% |
| `BenchmarkDetectCircleOverlapsGridLargeColliders/grid/10/large-colliders-16` | 30999.4 → 27555 | -11.1% | 15416 → 15416 | +0.0% | 133 → 133 | +0.0% |
| `BenchmarkDetectCircleOverlapsGridLargeColliders/grid/25/large-colliders-16` | 78078.8 → 68366.2 | -12.4% | 36328 → 36328 | +0.0% | 353 → 353 | +0.0% |
| `BenchmarkDetectCircleOverlapsGridLargeColliders/grid/50/large-colliders-16` | 157463.6 → 135881.4 | -13.7% | 69176 → 69176 | +0.0% | 637 → 637 | +0.0% |
| `BenchmarkDetectCircleOverlapsGridLargeColliders/grid/75/large-colliders-16` | 294310.2 → 240789.6 | -18.2% | 133304 → 133304 | +0.0% | 1021 → 1021 | +0.0% |
| `BenchmarkDetectCircleOverlapsGridLargeColliders/grid/100/large-colliders-16` | 318236 → 268877 | -15.5% | 143576 → 143576 | +0.0% | 1269 → 1269 | +0.0% |
| `BenchmarkDetectCircleOverlapsGridLargeColliders/grid/1000/large-colliders-16` | 3350723.2 → 2537115 | -24.3% | 1182235 → 1182435.8 | +0.0% | 8096 → 8096 | +0.0% |
| `BenchmarkResolveCircleOverlapsGrid/resolve-grid/10/spread-16` | 7778.2 → 5916.4 | -23.9% | 3888 → 3888 | +0.0% | 46 → 46 | +0.0% |
| `BenchmarkResolveCircleOverlapsGrid/resolve-grid/25/spread-16` | 16295.4 → 13706 | -15.9% | 8384 → 8384 | +0.0% | 78 → 78 | +0.0% |
| `BenchmarkResolveCircleOverlapsGrid/resolve-grid/50/spread-16` | 43433.4 → 35380 | -18.5% | 23008 → 23008 | +0.0% | 162 → 162 | +0.0% |
| `BenchmarkResolveCircleOverlapsGrid/resolve-grid/75/spread-16` | 55023 → 47508.8 | -13.7% | 30624 → 30624 | +0.0% | 204 → 204 | +0.0% |
| `BenchmarkResolveCircleOverlapsGrid/resolve-grid/100/spread-16` | 60314 → 54937.6 | -8.9% | 31840 → 31840 | +0.0% | 232 → 232 | +0.0% |
| `BenchmarkResolveCircleOverlapsGrid/resolve-grid/1000/spread-16` | 985939 → 840794.4 | -14.7% | 503801.2 → 503801 | -0.0% | 2587 → 2587 | +0.0% |
| `BenchmarkWorldStep/step/10/spread-16` | 4020.2 → 3863 | -3.9% | 1776 → 1776 | +0.0% | 30 → 30 | +0.0% |
| `BenchmarkWorldStep/step/25/spread-16` | 10838 → 10131.2 | -6.5% | 4928 → 4928 | +0.0% | 64 → 64 | +0.0% |
| `BenchmarkWorldStep/step/50/spread-16` | 21723.8 → 21626.4 | -0.4% | 10608 → 10608 | +0.0% | 118 → 118 | +0.0% |
| `BenchmarkWorldStep/step/75/spread-16` | 37455.8 → 36247.6 | -3.2% | 19376 → 19376 | +0.0% | 172 → 172 | +0.0% |
| `BenchmarkWorldStep/step/100/spread-16` | 46436.6 → 43488 | -6.3% | 21408 → 21408 | +0.0% | 222 → 222 | +0.0% |
| `BenchmarkWorldStep/step/1000/spread-16` | 722540.4 → 730398 | +1.1% | 242392 → 242392 | +0.0% | 873 → 873 | +0.0% |
| `BenchmarkWorldClone/clone/10/spread-16` | 2901.6 → 2296.6 | -20.9% | 2296 → 2296 | +0.0% | 19 → 19 | +0.0% |
| `BenchmarkWorldClone/clone/25/spread-16` | 5194.8 → 3888.4 | -25.1% | 3960 → 3960 | +0.0% | 19 → 19 | +0.0% |
| `BenchmarkWorldClone/clone/50/spread-16` | 8975.6 → 6775.8 | -24.5% | 6840 → 6840 | +0.0% | 19 → 19 | +0.0% |
| `BenchmarkWorldClone/clone/75/spread-16` | 13756.2 → 11341.8 | -17.6% | 12728 → 12728 | +0.0% | 19 → 19 | +0.0% |
| `BenchmarkWorldClone/clone/100/spread-16` | 15058.6 → 13477.4 | -10.5% | 12728 → 12728 | +0.0% | 19 → 19 | +0.0% |
| `BenchmarkWorldClone/clone/1000/spread-16` | 212943 → 180478.6 | -15.2% | 206896.4 → 206897 | +0.0% | 25 → 25 | +0.0% |

