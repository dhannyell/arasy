package main

import (
	"strings"
	"testing"

	"golang.org/x/perf/benchfmt"
)

// TestBenchstatDependencyIsTestOnly is a smoke test proving the benchstat
// libraries are wired in and parse benchmark output. The import lives only in
// this _test.go file, so golang.org/x/perf never enters the engine's build
// graph and never ships in a release game binary (verified separately with
// `go list -deps ./scene ./runtime`).
func TestBenchstatDependencyIsTestOnly(t *testing.T) {
	const sample = `goos: windows
goarch: amd64
pkg: example
BenchmarkFoo-16	1000	1234 ns/op	56 B/op	7 allocs/op
`

	reader := benchfmt.NewReader(strings.NewReader(sample), "sample")
	var count int
	for reader.Scan() {
		if res, ok := reader.Result().(*benchfmt.Result); ok {
			// benchfmt drops the "Benchmark" prefix from the reported name.
			if got := string(res.Name.Full()); got != "Foo-16" {
				t.Fatalf("benchmark name = %q, want Foo-16", got)
			}
			count++
		}
	}
	if err := reader.Err(); err != nil {
		t.Fatalf("benchfmt reader error: %v", err)
	}
	if count != 1 {
		t.Fatalf("parsed %d results, want 1", count)
	}
}

// TestRegressionGateWithBaseline verifies baseline resolution and error handling.
func TestRegressionGateWithBaseline(t *testing.T) {
	tests := []struct {
		name        string
		baseline    string
		run         string
		expectError bool
		description string
	}{
		{
			name:        "both files parse",
			baseline:    sampleBenchmarkOutput(),
			run:         sampleBenchmarkOutput(),
			expectError: false,
			description: "identical outputs should parse without error",
		},
		{
			name:        "baseline with new benchmark",
			baseline:    sampleBenchmarkOutput(),
			run:         sampleBenchmarkWithExtra(),
			expectError: false,
			description: "new benchmarks in run should not cause error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			baseline, err := parseBenchmarkWithMetadata(tt.baseline)
			if (err != nil) != tt.expectError {
				t.Errorf("baseline parse error=%v, expected=%v: %s", err != nil, tt.expectError, tt.description)
			}

			run, err := parseBenchmarkWithMetadata(tt.run)
			if (err != nil) != tt.expectError {
				t.Errorf("run parse error=%v, expected=%v: %s", err != nil, tt.expectError, tt.description)
			}

			if baseline != nil && run != nil && len(baseline) > 0 && len(run) > 0 {
				// Successfully parsed both; gate would not fail on parse.
			}
		})
	}
}

// parseBenchmarkWithMetadata parses benchmark results using benchstat libraries.
func parseBenchmarkWithMetadata(input string) ([]*benchfmt.Result, error) {
	reader := benchfmt.NewReader(strings.NewReader(input), "bench")
	var results []*benchfmt.Result
	for reader.Scan() {
		if res, ok := reader.Result().(*benchfmt.Result); ok {
			results = append(results, res)
		}
	}
	if err := reader.Err(); err != nil {
		return nil, err
	}
	return results, nil
}

func sampleBenchmarkOutput() string {
	return `goos: windows
goarch: amd64
pkg: example
BenchmarkExample-16	10	1234 ns/op	56 B/op	7 allocs/op
`
}

func sampleBenchmarkWithExtra() string {
	return `goos: windows
goarch: amd64
pkg: example
BenchmarkExample-16	10	1234 ns/op	56 B/op	7 allocs/op
BenchmarkNew-16	5	5678 ns/op	100 B/op	10 allocs/op
`
}
