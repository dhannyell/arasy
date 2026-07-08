package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseBenchmarkResults(t *testing.T) {
	output := strings.Join([]string{
		"goos: windows",
		"goarch: amd64",
		"BenchmarkWorldStep/step/10/spread-16          279393      3715 ns/op    1776 B/op      30 allocs/op",
		"BenchmarkRendererRender-16                       55  21390475 ns/op  3686920 B/op     521 allocs/op",
		"PASS",
	}, "\n")

	results := parseBenchmarkResults(output)
	if len(results) != 2 {
		t.Fatalf("len(results) = %d, want 2", len(results))
	}

	got := results[0]
	if got.Name != "BenchmarkWorldStep/step/10/spread-16" ||
		got.Runs != "279393" ||
		got.NsPerOp != "3715" ||
		got.BytesOp != "1776" ||
		got.AllocsOp != "30" {
		t.Fatalf("first result = %+v", got)
	}
}

func TestApplyRunProfile(t *testing.T) {
	tests := []struct {
		name          string
		profile       string
		count         int
		countSet      bool
		benchtime     string
		benchtimeSet  bool
		wantCount     int
		wantBenchtime string
		wantErr       bool
	}{
		{name: "empty", count: 1, wantCount: 1},
		{name: "quick", profile: "quick", count: 1, wantCount: 1},
		{name: "ci", profile: "ci", count: 1, wantCount: 3, wantBenchtime: "1s"},
		{name: "decision", profile: "decision", count: 1, wantCount: 5, wantBenchtime: "2s"},
		{name: "explicit values win", profile: "decision", count: 7, countSet: true, benchtime: "300x", benchtimeSet: true, wantCount: 7, wantBenchtime: "300x"},
		{name: "explicit count one wins", profile: "decision", count: 1, countSet: true, wantCount: 1, wantBenchtime: "2s"},
		{name: "unknown", profile: "slow", count: 1, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := applyRunProfile(tt.profile, tt.count, tt.countSet, tt.benchtime, tt.benchtimeSet)
			if tt.wantErr {
				if err == nil {
					t.Fatal("applyRunProfile returned nil error")
				}
				return
			}
			if err != nil {
				t.Fatalf("applyRunProfile returned error: %v", err)
			}
			if got.Count != tt.wantCount || got.Benchtime != tt.wantBenchtime {
				t.Fatalf("applyRunProfile = %+v, want count=%d benchtime=%q", got, tt.wantCount, tt.wantBenchtime)
			}
		})
	}
}

func TestRenderBenchmarkReportIncludesResultsAndRawOutput(t *testing.T) {
	report := newBenchmarkReport(
		time.Date(2026, 7, 8, 12, 0, 0, 0, time.UTC),
		1500*time.Millisecond,
		[]string{"test", "./scene", "-run", "^$", "-bench", ".", "-count", "1"},
		"BenchmarkWorldStep-16  10  100 ns/op  8 B/op  1 allocs/op\n",
		nil,
	)

	got := renderBenchmarkReport(report)
	for _, want := range []string{
		"- Status: `pass`",
		"| `BenchmarkWorldStep-16` | 10 | 100 | 8 | 1 |",
		"BenchmarkWorldStep-16  10  100 ns/op",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("rendered report missing %q:\n%s", want, got)
		}
	}
}

func TestWriteJSONReport(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bench-scene.json")
	report := benchmarkReport{
		Status:  "pass",
		Results: []benchmarkResult{{Name: "BenchmarkWorldStep-16", Runs: "10", NsPerOp: "100"}},
	}

	if err := writeJSONReport(path, report); err != nil {
		t.Fatalf("writeJSONReport returned error: %v", err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read JSON report: %v", err)
	}
	if !strings.Contains(string(content), `"ns_per_op": "100"`) {
		t.Fatalf("JSON report missing benchmark metric:\n%s", string(content))
	}
}
