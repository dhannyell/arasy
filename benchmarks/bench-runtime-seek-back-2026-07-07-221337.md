# Benchmark Report

## Metadata

- Status: `pass`
- Created at: `2026-07-07T22:13:37-03:00`
- Duration: `4m37.373s`
- OS: `windows`
- Arch: `amd64`
- CPU count: `16`
- Command: `go test ./runtime -run ^$ -bench BenchmarkDriverBack$ -count 1 -benchmem -benchtime 500x`

## Results

| Benchmark | Runs | ns/op | B/op | allocs/op |
| --- | ---: | ---: | ---: | ---: |
| `BenchmarkDriverBack/entities=100-16` | 500 | 34901248 | 14837843 | 143760 |
| `BenchmarkDriverBack/entities=1000-16` | 500 | 512655615 | 147946929 | 539537 |

## Raw Output

```text
goos: windows
goarch: amd64
pkg: github.com/dhannyell/frameforge/runtime
cpu: AMD Ryzen 7 5800X3D 8-Core Processor           
BenchmarkDriverBack/entities=100-16         	     500	  34901248 ns/op	14837843 B/op	  143760 allocs/op
BenchmarkDriverBack/entities=1000-16        	     500	 512655615 ns/op	147946929 B/op	  539537 allocs/op
PASS
ok  	github.com/dhannyell/frameforge/runtime	276.804s
```
