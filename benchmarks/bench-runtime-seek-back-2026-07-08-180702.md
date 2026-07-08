# Benchmark Report

## Metadata

- Status: `pass`
- Created at: `2026-07-08T18:07:02-03:00`
- Duration: `4m22.919s`
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
| `BenchmarkDriverBack/entities=100-16` | 500 | 30721554 | 14835465 | 143796 |
| `BenchmarkDriverBack/entities=1000-16` | 500 | 486709168 | 147824282 | 539665 |

## Raw Output

```text
goos: windows
goarch: amd64
pkg: github.com/dhannyell/frameforge/runtime
cpu: AMD Ryzen 7 5800X3D 8-Core Processor           
BenchmarkDriverBack/entities=100-16         	     500	  30721554 ns/op	14835465 B/op	  143796 allocs/op
BenchmarkDriverBack/entities=1000-16        	     500	 486709168 ns/op	147824282 B/op	  539665 allocs/op
PASS
ok  	github.com/dhannyell/frameforge/runtime	262.202s
```
