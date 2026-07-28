package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIsBenchmarkReportName(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{name: "bench-scene-2026-07-08-120000.md", want: true},
		{name: "bench-runtime-seek-2026-07-08.md", want: true},
		{name: "compare.md", want: false},
		{name: "compare-2026-07-08.md", want: false},
		{name: "bench-scene-2026-07-08-120000.json", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isBenchmarkReportName(tt.name); got != tt.want {
				t.Fatalf("isBenchmarkReportName(%q) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

func TestFindPreviousReportIgnoresComparisonReports(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{
		"compare.md",
		"bench-scene-2026-07-08-110000.md",
		"bench-scene-2026-07-08-120000.md",
		"bench-scene-2026-07-08-120000.json",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(""), 0o644); err != nil {
			t.Fatalf("failed to write %s: %v", name, err)
		}
	}

	got, err := findPreviousReport(dir, "scene")
	if err != nil {
		t.Fatalf("findPreviousReport returned error: %v", err)
	}
	if filepath.Base(got) != "bench-scene-2026-07-08-120000.md" {
		t.Fatalf("findPreviousReport = %s", got)
	}
}

func TestFindPreviousReportMatchesExactFamily(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{
		"bench-runtime-seek-2026-07-08-110000.md",
		"bench-runtime-seek-back-2026-07-08-120000.md",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(""), 0o644); err != nil {
			t.Fatalf("failed to write %s: %v", name, err)
		}
	}

	got, err := findPreviousReport(dir, "runtime-seek")
	if err != nil {
		t.Fatalf("findPreviousReport returned error: %v", err)
	}
	if filepath.Base(got) != "bench-runtime-seek-2026-07-08-110000.md" {
		t.Fatalf("findPreviousReport = %s", got)
	}
}

func TestBaselineRootDirUsesParentForRunsDir(t *testing.T) {
	tests := []struct {
		name string
		out  string
		want string
	}{
		{name: "default runs", out: filepath.Join("benchmarks", "runs"), want: "benchmarks"},
		{name: "custom regular dir", out: "tmp-benches", want: "tmp-benches"},
		{name: "custom runs dir", out: filepath.Join("tmp-benches", "runs"), want: "tmp-benches"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := baselineRootDir(tt.out); got != tt.want {
				t.Fatalf("baselineRootDir(%q) = %q, want %q", tt.out, got, tt.want)
			}
		})
	}
}

func TestNormalizeBenchmarkOutputRemovesFormattingWhitespace(t *testing.T) {
	input := "goos: windows\r\ncpu: Example CPU   \r\nBenchmarkA-16  10  100 ns/op\t\n"
	want := "goos: windows\ncpu: Example CPU\nBenchmarkA-16  10  100 ns/op\n"

	if got := normalizeBenchmarkOutput(input); got != want {
		t.Fatalf("normalizeBenchmarkOutput() = %q, want %q", got, want)
	}
}

func TestBuildComparisonAggregatesRepeatedRuns(t *testing.T) {
	oldResults := []benchmarkResult{
		{Name: "BenchmarkA-16", NsPerOp: "100", BytesOp: "10", AllocsOp: "1"},
		{Name: "BenchmarkA-16", NsPerOp: "140", BytesOp: "14", AllocsOp: "3"},
	}
	newResults := []benchmarkResult{
		{Name: "BenchmarkA-16", NsPerOp: "60", BytesOp: "6", AllocsOp: "1"},
		{Name: "BenchmarkA-16", NsPerOp: "80", BytesOp: "8", AllocsOp: "1"},
	}

	rows := buildComparison(oldResults, newResults)
	if len(rows) != 1 {
		t.Fatalf("len(rows) = %d, want 1", len(rows))
	}
	row := rows[0]
	if row.OldNs != 120 || row.NewNs != 70 || row.OldBytes != 12 || row.NewBytes != 7 || row.OldAllocs != 2 || row.NewAllocs != 1 {
		t.Fatalf("row = %+v", row)
	}
}

func TestRenderComparisonTableIncludesSummary(t *testing.T) {
	rows := []comparisonRow{
		{Name: "BenchmarkSlower-16", HasOld: true, HasNew: true, OldNs: 100, NewNs: 125},
		{Name: "BenchmarkFaster-16", HasOld: true, HasNew: true, OldNs: 100, NewNs: 80},
		{Name: "BenchmarkNew-16", HasNew: true, NewNs: 50},
		{Name: "BenchmarkRemoved-16", HasOld: true, OldNs: 50},
	}

	got := renderComparisonTable(rows)
	for _, want := range []string{
		"### Summary",
		"- Benchmarks: `2` compared, `1` new, `1` removed",
		"`BenchmarkSlower-16` (+25.0%)",
		"`BenchmarkFaster-16` (-20.0%)",
		"| `BenchmarkNew-16` | — → 50 | new |",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("comparison table missing %q:\n%s", want, got)
		}
	}
}
