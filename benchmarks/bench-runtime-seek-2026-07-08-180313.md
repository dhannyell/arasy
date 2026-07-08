# Benchmark Report

## Metadata

- Status: `pass`
- Created at: `2026-07-08T18:03:13-03:00`
- Duration: `1m33.367s`
- OS: `windows`
- Arch: `amd64`
- CPU count: `16`
- Commit SHA: `e5e39e8`
- Go version: `go1.26.4`
- GOMAXPROCS: `16`
- Command: `go test ./runtime -run ^$ -bench BenchmarkDriver(Step|ScrubToTick)$|BenchmarkReplay$ -count 1 -benchmem`

## Results

| Benchmark | Runs | ns/op | B/op | allocs/op |
| --- | ---: | ---: | ---: | ---: |
| `BenchmarkDriverStep/entities=100/history=0-16` | 28172 | 43147 | 24389 | 226 |
| `BenchmarkDriverStep/entities=100/history=600-16` | 27331 | 48174 | 24895 | 228 |
| `BenchmarkDriverStep/entities=1000/history=0-16` | 1543 | 725113 | 246364 | 902 |
| `BenchmarkDriverStep/entities=1000/history=600-16` | 1495 | 791499 | 250024 | 903 |
| `BenchmarkDriverScrubToTick/entities=100/timeline=600/to-1-16` | 32039643 | 53.80 | 24 | 1 |
| `BenchmarkDriverScrubToTick/entities=100/timeline=600/to-half-16` | 21427508 | 53.70 | 24 | 1 |
| `BenchmarkDriverScrubToTick/entities=100/timeline=600/to-last-16` | 23594833 | 50.14 | 24 | 1 |
| `BenchmarkDriverScrubToTick/entities=100/timeline=3600/to-1-16` | 29222209 | 46.19 | 24 | 1 |
| `BenchmarkDriverScrubToTick/entities=100/timeline=3600/to-half-16` | 24153276 | 47.28 | 25 | 1 |
| `BenchmarkDriverScrubToTick/entities=100/timeline=3600/to-last-16` | 20783109 | 48.71 | 28 | 1 |
| `BenchmarkDriverScrubToTick/entities=1000/timeline=600/to-1-16` | 26784936 | 44.88 | 24 | 1 |
| `BenchmarkDriverScrubToTick/entities=1000/timeline=600/to-half-16` | 23101566 | 48.12 | 27 | 1 |
| `BenchmarkDriverScrubToTick/entities=1000/timeline=600/to-last-16` | 15485032 | 65.69 | 33 | 1 |
| `BenchmarkDriverScrubToTick/entities=1000/timeline=3600/to-1-16` | 29564732 | 38.17 | 24 | 1 |
| `BenchmarkDriverScrubToTick/entities=1000/timeline=3600/to-half-16` | 1 | 1290937800 | 444151672 | 1630095 |
| `BenchmarkDriverScrubToTick/entities=1000/timeline=3600/to-last-16` | 1 | 2609625900 | 887936432 | 3269122 |
| `BenchmarkReplay/entities=100/ticks=600-16` | 40 | 29971572 | 14873470 | 144122 |
| `BenchmarkReplay/entities=1000/ticks=600-16` | 3 | 462109000 | 148095053 | 540673 |

## Raw Output

```text
goos: windows
goarch: amd64
pkg: github.com/dhannyell/frameforge/runtime
cpu: AMD Ryzen 7 5800X3D 8-Core Processor           
BenchmarkDriverStep/entities=100/history=0-16         	   28172	     43147 ns/op	   24389 B/op	     226 allocs/op
BenchmarkDriverStep/entities=100/history=600-16       	   27331	     48174 ns/op	   24895 B/op	     228 allocs/op
BenchmarkDriverStep/entities=1000/history=0-16        	    1543	    725113 ns/op	  246364 B/op	     902 allocs/op
BenchmarkDriverStep/entities=1000/history=600-16      	    1495	    791499 ns/op	  250024 B/op	     903 allocs/op
BenchmarkDriverScrubToTick/entities=100/timeline=600/to-1-16         	32039643	        53.80 ns/op	      24 B/op	       1 allocs/op
BenchmarkDriverScrubToTick/entities=100/timeline=600/to-half-16      	21427508	        53.70 ns/op	      24 B/op	       1 allocs/op
BenchmarkDriverScrubToTick/entities=100/timeline=600/to-last-16      	23594833	        50.14 ns/op	      24 B/op	       1 allocs/op
BenchmarkDriverScrubToTick/entities=100/timeline=3600/to-1-16        	29222209	        46.19 ns/op	      24 B/op	       1 allocs/op
BenchmarkDriverScrubToTick/entities=100/timeline=3600/to-half-16     	24153276	        47.28 ns/op	      25 B/op	       1 allocs/op
BenchmarkDriverScrubToTick/entities=100/timeline=3600/to-last-16     	20783109	        48.71 ns/op	      28 B/op	       1 allocs/op
BenchmarkDriverScrubToTick/entities=1000/timeline=600/to-1-16        	26784936	        44.88 ns/op	      24 B/op	       1 allocs/op
BenchmarkDriverScrubToTick/entities=1000/timeline=600/to-half-16     	23101566	        48.12 ns/op	      27 B/op	       1 allocs/op
BenchmarkDriverScrubToTick/entities=1000/timeline=600/to-last-16     	15485032	        65.69 ns/op	      33 B/op	       1 allocs/op
BenchmarkDriverScrubToTick/entities=1000/timeline=3600/to-1-16       	29564732	        38.17 ns/op	      24 B/op	       1 allocs/op
BenchmarkDriverScrubToTick/entities=1000/timeline=3600/to-half-16    	       1	1290937800 ns/op	444151672 B/op	 1630095 allocs/op
BenchmarkDriverScrubToTick/entities=1000/timeline=3600/to-last-16    	       1	2609625900 ns/op	887936432 B/op	 3269122 allocs/op
BenchmarkReplay/entities=100/ticks=600-16                            	      40	  29971572 ns/op	14873470 B/op	  144122 allocs/op
BenchmarkReplay/entities=1000/ticks=600-16                           	       3	 462109000 ns/op	148095053 B/op	  540673 allocs/op
PASS
ok  	github.com/dhannyell/frameforge/runtime	92.663s
```
