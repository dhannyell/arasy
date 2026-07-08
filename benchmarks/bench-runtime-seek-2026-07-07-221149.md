# Benchmark Report

## Metadata

- Status: `pass`
- Created at: `2026-07-07T22:11:49-03:00`
- Duration: `1m48s`
- OS: `windows`
- Arch: `amd64`
- CPU count: `16`
- Command: `go test ./runtime -run ^$ -bench BenchmarkDriver(Step|ScrubToTick)$|BenchmarkReplay$ -count 1 -benchmem`

## Results

| Benchmark | Runs | ns/op | B/op | allocs/op |
| --- | ---: | ---: | ---: | ---: |
| `BenchmarkDriverStep/entities=100/history=0-16` | 21930 | 52973 | 24389 | 226 |
| `BenchmarkDriverStep/entities=100/history=600-16` | 21763 | 58971 | 25118 | 228 |
| `BenchmarkDriverStep/entities=1000/history=0-16` | 1392 | 966881 | 246395 | 902 |
| `BenchmarkDriverStep/entities=1000/history=600-16` | 1386 | 868535 | 256014 | 904 |
| `BenchmarkDriverScrubToTick/entities=100/timeline=600/to-1-16` | 20569254 | 55.85 | 24 | 1 |
| `BenchmarkDriverScrubToTick/entities=100/timeline=600/to-half-16` | 24200028 | 56.50 | 24 | 1 |
| `BenchmarkDriverScrubToTick/entities=100/timeline=600/to-last-16` | 23456810 | 48.69 | 24 | 1 |
| `BenchmarkDriverScrubToTick/entities=100/timeline=3600/to-1-16` | 26466637 | 43.13 | 24 | 1 |
| `BenchmarkDriverScrubToTick/entities=100/timeline=3600/to-half-16` | 21503720 | 53.58 | 26 | 1 |
| `BenchmarkDriverScrubToTick/entities=100/timeline=3600/to-last-16` | 23423426 | 51.61 | 27 | 1 |
| `BenchmarkDriverScrubToTick/entities=1000/timeline=600/to-1-16` | 34935108 | 36.16 | 24 | 1 |
| `BenchmarkDriverScrubToTick/entities=1000/timeline=600/to-half-16` | 24627099 | 48.52 | 27 | 1 |
| `BenchmarkDriverScrubToTick/entities=1000/timeline=600/to-last-16` | 15118881 | 69.26 | 33 | 1 |
| `BenchmarkDriverScrubToTick/entities=1000/timeline=3600/to-1-16` | 36182721 | 34.85 | 24 | 1 |
| `BenchmarkDriverScrubToTick/entities=1000/timeline=3600/to-half-16` | 1 | 1512688300 | 444106216 | 1629789 |
| `BenchmarkDriverScrubToTick/entities=1000/timeline=3600/to-last-16` | 1 | 3005597800 | 887631224 | 3268533 |
| `BenchmarkReplay/entities=100/ticks=600-16` | 37 | 34472024 | 14877057 | 144090 |
| `BenchmarkReplay/entities=1000/ticks=600-16` | 2 | 524499750 | 148317792 | 540639 |

## Raw Output

```text
goos: windows
goarch: amd64
pkg: github.com/dhannyell/frameforge/runtime
cpu: AMD Ryzen 7 5800X3D 8-Core Processor           
BenchmarkDriverStep/entities=100/history=0-16         	   21930	     52973 ns/op	   24389 B/op	     226 allocs/op
BenchmarkDriverStep/entities=100/history=600-16       	   21763	     58971 ns/op	   25118 B/op	     228 allocs/op
BenchmarkDriverStep/entities=1000/history=0-16        	    1392	    966881 ns/op	  246395 B/op	     902 allocs/op
BenchmarkDriverStep/entities=1000/history=600-16      	    1386	    868535 ns/op	  256014 B/op	     904 allocs/op
BenchmarkDriverScrubToTick/entities=100/timeline=600/to-1-16         	20569254	        55.85 ns/op	      24 B/op	       1 allocs/op
BenchmarkDriverScrubToTick/entities=100/timeline=600/to-half-16      	24200028	        56.50 ns/op	      24 B/op	       1 allocs/op
BenchmarkDriverScrubToTick/entities=100/timeline=600/to-last-16      	23456810	        48.69 ns/op	      24 B/op	       1 allocs/op
BenchmarkDriverScrubToTick/entities=100/timeline=3600/to-1-16        	26466637	        43.13 ns/op	      24 B/op	       1 allocs/op
BenchmarkDriverScrubToTick/entities=100/timeline=3600/to-half-16     	21503720	        53.58 ns/op	      26 B/op	       1 allocs/op
BenchmarkDriverScrubToTick/entities=100/timeline=3600/to-last-16     	23423426	        51.61 ns/op	      27 B/op	       1 allocs/op
BenchmarkDriverScrubToTick/entities=1000/timeline=600/to-1-16        	34935108	        36.16 ns/op	      24 B/op	       1 allocs/op
BenchmarkDriverScrubToTick/entities=1000/timeline=600/to-half-16     	24627099	        48.52 ns/op	      27 B/op	       1 allocs/op
BenchmarkDriverScrubToTick/entities=1000/timeline=600/to-last-16     	15118881	        69.26 ns/op	      33 B/op	       1 allocs/op
BenchmarkDriverScrubToTick/entities=1000/timeline=3600/to-1-16       	36182721	        34.85 ns/op	      24 B/op	       1 allocs/op
BenchmarkDriverScrubToTick/entities=1000/timeline=3600/to-half-16    	       1	1512688300 ns/op	444106216 B/op	 1629789 allocs/op
BenchmarkDriverScrubToTick/entities=1000/timeline=3600/to-last-16    	       1	3005597800 ns/op	887631224 B/op	 3268533 allocs/op
BenchmarkReplay/entities=100/ticks=600-16                            	      37	  34472024 ns/op	14877057 B/op	  144090 allocs/op
BenchmarkReplay/entities=1000/ticks=600-16                           	       2	 524499750 ns/op	148317792 B/op	  540639 allocs/op
PASS
ok  	github.com/dhannyell/frameforge/runtime	107.401s
```
