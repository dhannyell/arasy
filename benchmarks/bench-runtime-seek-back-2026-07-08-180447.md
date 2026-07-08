# Benchmark Report

## Metadata

- Status: `pass`
- Created at: `2026-07-08T18:04:47-03:00`
- Duration: `4m19.054s`
- OS: `windows`
- Arch: `amd64`
- CPU count: `16`
- Commit SHA: `e5e39e8`
- Go version: `go1.26.4`
- GOMAXPROCS: `16`
- Command: `go test ./runtime -run ^$ -bench BenchmarkDriverBack$ -count 1 -benchmem -benchtime 500x`

## Results

| Benchmark | Runs | ns/op | B/op | allocs/op |
| --- | ---: | ---: | ---: | ---: |
| `BenchmarkDriverBack/entities=100-16` | 500 | 27485550 | 14835414 | 143796 |
| `BenchmarkDriverBack/entities=1000-16` | 500 | 482716718 | 147823862 | 539665 |

## Raw Output

```text
goos: windows
goarch: amd64
pkg: github.com/dhannyell/frameforge/runtime
cpu: AMD Ryzen 7 5800X3D 8-Core Processor           
BenchmarkDriverBack/entities=100-16         	     500	  27485550 ns/op	14835414 B/op	  143796 allocs/op
BenchmarkDriverBack/entities=1000-16        	     500	 482716718 ns/op	147823862 B/op	  539665 allocs/op
PASS
ok  	github.com/dhannyell/frameforge/runtime	258.330s
```
