# Benchmark Report

## Metadata

- Status: `pass`
- Created at: `2026-07-08T15:50:24-03:00`
- Duration: `4m7.75s`
- OS: `windows`
- Arch: `amd64`
- CPU count: `16`
- Command: `go test ./runtime -run ^$ -bench BenchmarkDriverBack$ -count 1 -benchmem -benchtime 500x`

## Results

| Benchmark | Runs | ns/op | B/op | allocs/op |
| --- | ---: | ---: | ---: | ---: |
| `BenchmarkDriverBack/entities=100-16` | 500 | 25000898 | 14835290 | 143796 |
| `BenchmarkDriverBack/entities=1000-16` | 500 | 461970306 | 147821840 | 539663 |

## Raw Output

```text
goos: windows
goarch: amd64
pkg: github.com/dhannyell/frameforge/runtime
cpu: AMD Ryzen 7 5800X3D 8-Core Processor           
BenchmarkDriverBack/entities=100-16         	     500	  25000898 ns/op	14835290 B/op	  143796 allocs/op
BenchmarkDriverBack/entities=1000-16        	     500	 461970306 ns/op	147821840 B/op	  539663 allocs/op
PASS
ok  	github.com/dhannyell/frameforge/runtime	247.113s
```

## Comparison: bench-runtime-seek-back-2026-07-07-221337.md → bench-runtime-seek-back-2026-07-08-155024.md

Negative % = faster/smaller (improvement). Positive % = slower/bigger (regression).

### Summary

- Benchmarks: `2` compared, `0` new, `0` removed
- Regressions >= 10% ns/op: none
- Improvements >= 10% ns/op: `BenchmarkDriverBack/entities=100-16` (-28.4%)

| Benchmark | ns/op (old → new) | Δ ns/op | B/op (old → new) | Δ B/op | allocs/op (old → new) | Δ allocs/op |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| `BenchmarkDriverBack/entities=100-16` | 34901248 → 25000898 | -28.4% | 14837843 → 14835290 | -0.0% | 143760 → 143796 | +0.0% |
| `BenchmarkDriverBack/entities=1000-16` | 512655615 → 461970306 | -9.9% | 147946929 → 147821840 | -0.1% | 539537 → 539663 | +0.0% |
