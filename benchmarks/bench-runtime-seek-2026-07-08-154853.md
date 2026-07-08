# Benchmark Report

## Metadata

- Status: `pass`
- Created at: `2026-07-08T15:48:53-03:00`
- Duration: `1m31.201s`
- OS: `windows`
- Arch: `amd64`
- CPU count: `16`
- Command: `go test ./runtime -run ^$ -bench BenchmarkDriver(Step|ScrubToTick)$|BenchmarkReplay$ -count 1 -benchmem`

## Results

| Benchmark | Runs | ns/op | B/op | allocs/op |
| --- | ---: | ---: | ---: | ---: |
| `BenchmarkDriverStep/entities=100/history=0-16` | 29538 | 41530 | 24388 | 226 |
| `BenchmarkDriverStep/entities=100/history=600-16` | 28250 | 42156 | 24894 | 228 |
| `BenchmarkDriverStep/entities=1000/history=0-16` | 1716 | 669335 | 246331 | 902 |
| `BenchmarkDriverStep/entities=1000/history=600-16` | 1833 | 703661 | 250040 | 903 |
| `BenchmarkDriverScrubToTick/entities=100/timeline=600/to-1-16` | 34850085 | 59.52 | 24 | 1 |
| `BenchmarkDriverScrubToTick/entities=100/timeline=600/to-half-16` | 21807182 | 64.06 | 24 | 1 |
| `BenchmarkDriverScrubToTick/entities=100/timeline=600/to-last-16` | 14287389 | 74.60 | 25 | 1 |
| `BenchmarkDriverScrubToTick/entities=100/timeline=3600/to-1-16` | 21814809 | 53.24 | 24 | 1 |
| `BenchmarkDriverScrubToTick/entities=100/timeline=3600/to-half-16` | 16354254 | 64.93 | 26 | 1 |
| `BenchmarkDriverScrubToTick/entities=100/timeline=3600/to-last-16` | 19499599 | 58.83 | 28 | 1 |
| `BenchmarkDriverScrubToTick/entities=1000/timeline=600/to-1-16` | 25731636 | 46.35 | 24 | 1 |
| `BenchmarkDriverScrubToTick/entities=1000/timeline=600/to-half-16` | 24067209 | 51.54 | 27 | 1 |
| `BenchmarkDriverScrubToTick/entities=1000/timeline=600/to-last-16` | 16027365 | 65.35 | 33 | 1 |
| `BenchmarkDriverScrubToTick/entities=1000/timeline=3600/to-1-16` | 31974746 | 36.11 | 24 | 1 |
| `BenchmarkDriverScrubToTick/entities=1000/timeline=3600/to-half-16` | 1 | 1270804400 | 444194552 | 1630140 |
| `BenchmarkDriverScrubToTick/entities=1000/timeline=3600/to-last-16` | 1 | 2462719300 | 887904448 | 3269084 |
| `BenchmarkReplay/entities=100/ticks=600-16` | 48 | 28047669 | 14873005 | 144121 |
| `BenchmarkReplay/entities=1000/ticks=600-16` | 3 | 403850433 | 148071072 | 540659 |

## Raw Output

```text
goos: windows
goarch: amd64
pkg: github.com/dhannyell/frameforge/runtime
cpu: AMD Ryzen 7 5800X3D 8-Core Processor           
BenchmarkDriverStep/entities=100/history=0-16         	   29538	     41530 ns/op	   24388 B/op	     226 allocs/op
BenchmarkDriverStep/entities=100/history=600-16       	   28250	     42156 ns/op	   24894 B/op	     228 allocs/op
BenchmarkDriverStep/entities=1000/history=0-16        	    1716	    669335 ns/op	  246331 B/op	     902 allocs/op
BenchmarkDriverStep/entities=1000/history=600-16      	    1833	    703661 ns/op	  250040 B/op	     903 allocs/op
BenchmarkDriverScrubToTick/entities=100/timeline=600/to-1-16         	34850085	        59.52 ns/op	      24 B/op	       1 allocs/op
BenchmarkDriverScrubToTick/entities=100/timeline=600/to-half-16      	21807182	        64.06 ns/op	      24 B/op	       1 allocs/op
BenchmarkDriverScrubToTick/entities=100/timeline=600/to-last-16      	14287389	        74.60 ns/op	      25 B/op	       1 allocs/op
BenchmarkDriverScrubToTick/entities=100/timeline=3600/to-1-16        	21814809	        53.24 ns/op	      24 B/op	       1 allocs/op
BenchmarkDriverScrubToTick/entities=100/timeline=3600/to-half-16     	16354254	        64.93 ns/op	      26 B/op	       1 allocs/op
BenchmarkDriverScrubToTick/entities=100/timeline=3600/to-last-16     	19499599	        58.83 ns/op	      28 B/op	       1 allocs/op
BenchmarkDriverScrubToTick/entities=1000/timeline=600/to-1-16        	25731636	        46.35 ns/op	      24 B/op	       1 allocs/op
BenchmarkDriverScrubToTick/entities=1000/timeline=600/to-half-16     	24067209	        51.54 ns/op	      27 B/op	       1 allocs/op
BenchmarkDriverScrubToTick/entities=1000/timeline=600/to-last-16     	16027365	        65.35 ns/op	      33 B/op	       1 allocs/op
BenchmarkDriverScrubToTick/entities=1000/timeline=3600/to-1-16       	31974746	        36.11 ns/op	      24 B/op	       1 allocs/op
BenchmarkDriverScrubToTick/entities=1000/timeline=3600/to-half-16    	       1	1270804400 ns/op	444194552 B/op	 1630140 allocs/op
BenchmarkDriverScrubToTick/entities=1000/timeline=3600/to-last-16    	       1	2462719300 ns/op	887904448 B/op	 3269084 allocs/op
BenchmarkReplay/entities=100/ticks=600-16                            	      48	  28047669 ns/op	14873005 B/op	  144121 allocs/op
BenchmarkReplay/entities=1000/ticks=600-16                           	       3	 403850433 ns/op	148071072 B/op	  540659 allocs/op
PASS
ok  	github.com/dhannyell/frameforge/runtime	90.597s
```

## Comparison: bench-runtime-seek-back-2026-07-07-221337.md → bench-runtime-seek-2026-07-08-154853.md

Negative % = faster/smaller (improvement). Positive % = slower/bigger (regression).

### Summary

- Benchmarks: `0` compared, `18` new, `2` removed
- Regressions >= 10% ns/op: none
- Improvements >= 10% ns/op: none

| Benchmark | ns/op (old → new) | Δ ns/op | B/op (old → new) | Δ B/op | allocs/op (old → new) | Δ allocs/op |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| `BenchmarkDriverStep/entities=100/history=0-16` | — → 41530 | novo | — → 24388 | novo | — → 226 | novo |
| `BenchmarkDriverStep/entities=100/history=600-16` | — → 42156 | novo | — → 24894 | novo | — → 228 | novo |
| `BenchmarkDriverStep/entities=1000/history=0-16` | — → 669335 | novo | — → 246331 | novo | — → 902 | novo |
| `BenchmarkDriverStep/entities=1000/history=600-16` | — → 703661 | novo | — → 250040 | novo | — → 903 | novo |
| `BenchmarkDriverScrubToTick/entities=100/timeline=600/to-1-16` | — → 59.52 | novo | — → 24 | novo | — → 1 | novo |
| `BenchmarkDriverScrubToTick/entities=100/timeline=600/to-half-16` | — → 64.06 | novo | — → 24 | novo | — → 1 | novo |
| `BenchmarkDriverScrubToTick/entities=100/timeline=600/to-last-16` | — → 74.6 | novo | — → 25 | novo | — → 1 | novo |
| `BenchmarkDriverScrubToTick/entities=100/timeline=3600/to-1-16` | — → 53.24 | novo | — → 24 | novo | — → 1 | novo |
| `BenchmarkDriverScrubToTick/entities=100/timeline=3600/to-half-16` | — → 64.93 | novo | — → 26 | novo | — → 1 | novo |
| `BenchmarkDriverScrubToTick/entities=100/timeline=3600/to-last-16` | — → 58.83 | novo | — → 28 | novo | — → 1 | novo |
| `BenchmarkDriverScrubToTick/entities=1000/timeline=600/to-1-16` | — → 46.35 | novo | — → 24 | novo | — → 1 | novo |
| `BenchmarkDriverScrubToTick/entities=1000/timeline=600/to-half-16` | — → 51.54 | novo | — → 27 | novo | — → 1 | novo |
| `BenchmarkDriverScrubToTick/entities=1000/timeline=600/to-last-16` | — → 65.35 | novo | — → 33 | novo | — → 1 | novo |
| `BenchmarkDriverScrubToTick/entities=1000/timeline=3600/to-1-16` | — → 36.11 | novo | — → 24 | novo | — → 1 | novo |
| `BenchmarkDriverScrubToTick/entities=1000/timeline=3600/to-half-16` | — → 1270804400 | novo | — → 444194552 | novo | — → 1630140 | novo |
| `BenchmarkDriverScrubToTick/entities=1000/timeline=3600/to-last-16` | — → 2462719300 | novo | — → 887904448 | novo | — → 3269084 | novo |
| `BenchmarkReplay/entities=100/ticks=600-16` | — → 28047669 | novo | — → 14873005 | novo | — → 144121 | novo |
| `BenchmarkReplay/entities=1000/ticks=600-16` | — → 403850433 | novo | — → 148071072 | novo | — → 540659 | novo |
| `BenchmarkDriverBack/entities=100-16` | 34901248 → — | removido | 14837843 → — | removido | 143760 → — | removido |
| `BenchmarkDriverBack/entities=1000-16` | 512655615 → — | removido | 147946929 → — | removido | 539537 → — | removido |
