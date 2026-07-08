package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// timestampSuffix matches the trailing timestamp benchsave appends to a
// report's default name (e.g. "-2026-07-02" or "-2026-07-02-023854"), so it
// can be stripped to recover the report "family" (e.g. "bench-scene",
// "bench-runtime-seek", "bench-runtime-seek-back").
var timestampSuffix = regexp.MustCompile(`-\d{4}-\d{2}-\d{2}(-\d{6})?$`)

func reportFamily(baseName string) string {
	family := timestampSuffix.ReplaceAllString(baseName, "")
	if family == "" {
		return baseName
	}
	return family
}

// comparisonRow holds the old/new numbers for one benchmark name, matched
// across two reports parsed with parseBenchmarkResults.
type comparisonRow struct {
	Name                 string
	HasOld, HasNew       bool
	OldNs, NewNs         float64
	OldBytes, NewBytes   float64
	OldAllocs, NewAllocs float64
}

// compareAgainstPrevious resolves which previous report to diff against
// (either the explicit path in compareFlag, or the most recent report for
// family in outDir when compareFlag is "auto"), diffs it against the results in
// the freshly captured `output`, and returns the rendered markdown section.
func compareAgainstPrevious(compareFlag, outDir, family, newReportLabel, output string) (string, error) {
	prevPath := compareFlag
	if prevPath == "auto" {
		found, err := findPreviousReport(outDir, family)
		if err != nil {
			return "", err
		}
		prevPath = found
	}

	oldResults, err := parseReportFile(prevPath)
	if err != nil {
		return "", err
	}

	newResults := parseBenchmarkResults(output)
	rows := buildComparison(oldResults, newResults)
	return renderComparison(filepath.Base(prevPath), newReportLabel, rows), nil
}

// runDiffOnly compares two already-saved reports without running any
// benchmarks, printing the comparison to stdout and, when outPath is
// non-empty, also saving it as its own markdown file.
func runDiffOnly(oldPath, newPath, outPath string) error {
	oldResults, err := parseReportFile(oldPath)
	if err != nil {
		return err
	}
	newResults, err := parseReportFile(newPath)
	if err != nil {
		return err
	}

	rows := buildComparison(oldResults, newResults)
	comparison := renderComparison(filepath.Base(oldPath), filepath.Base(newPath), rows)
	fmt.Print(comparison)

	if outPath != "" {
		if writeErr := os.WriteFile(outPath, []byte(comparison), 0o644); writeErr != nil {
			return fmt.Errorf("failed to write comparison report to %s: %w", outPath, writeErr)
		}
		fmt.Printf("\ncomparison report saved to %s\n", outPath)
	}

	return nil
}

// runCompareAll scans every report in outDir, groups them by family (the
// report name with its trailing timestamp stripped — see reportFamily), and
// for each family compares its two most recent reports. Families with only
// one report so far are listed as a baseline with nothing to compare yet.
// This is how a report like bench-runtime-seek-*.md (saved with a custom
// -name, so it never matches the -pkg-based prefix that -compare auto looks
// for) still shows up: every family found in outDir gets its own section.
func runCompareAll(outDir, outPath string) error {
	entries, err := os.ReadDir(outDir)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", outDir, err)
	}

	families := make(map[string][]string)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		base := strings.TrimSuffix(entry.Name(), ".md")
		fam := reportFamily(base)
		families[fam] = append(families[fam], entry.Name())
	}

	if len(families) == 0 {
		return fmt.Errorf("no reports found in %s", outDir)
	}

	famNames := make([]string, 0, len(families))
	for fam, files := range families {
		sort.Strings(files) // timestamp suffix sorts chronologically as a string
		families[fam] = files
		famNames = append(famNames, fam)
	}
	sort.Strings(famNames)

	var b strings.Builder
	fmt.Fprintf(&b, "# Benchmark Comparison Report\n\nGenerated at %s from every report family in `%s`.\n\n", time.Now().Format(time.RFC3339), outDir)

	for _, fam := range famNames {
		files := families[fam]
		fmt.Fprintf(&b, "## %s\n\n", fam)

		if len(files) < 2 {
			fmt.Fprintf(&b, "Only one report so far (`%s`) — nothing to compare yet, this is the baseline.\n\n", files[0])
			continue
		}

		oldName, newName := files[len(files)-2], files[len(files)-1]
		oldResults, err := parseReportFile(filepath.Join(outDir, oldName))
		if err != nil {
			fmt.Fprintf(&b, "Comparison skipped: %v\n\n", err)
			continue
		}
		newResults, err := parseReportFile(filepath.Join(outDir, newName))
		if err != nil {
			fmt.Fprintf(&b, "Comparison skipped: %v\n\n", err)
			continue
		}

		rows := buildComparison(oldResults, newResults)
		fmt.Fprintf(&b, "_%s → %s_\n\n", oldName, newName)
		b.WriteString(renderComparisonTable(rows))
		b.WriteString("\n")
	}

	report := b.String()
	fmt.Print(report)

	if outPath != "" {
		if writeErr := os.WriteFile(outPath, []byte(report), 0o644); writeErr != nil {
			return fmt.Errorf("failed to write comparison report to %s: %w", outPath, writeErr)
		}
		fmt.Printf("comparison report saved to %s\n", outPath)
	}

	return nil
}

// findPreviousReport returns the most recent report in outDir whose name was
// produced by the default "bench-<family>-<timestamp>.md" naming. It is
// meant to be called before the current run's report is written, so it never
// picks up the report being generated in this run.
func findPreviousReport(outDir, family string) (string, error) {
	prefix := "bench-" + sanitizeName(family) + "-"

	entries, err := os.ReadDir(outDir)
	if err != nil {
		return "", fmt.Errorf("failed to read %s: %w", outDir, err)
	}

	var candidates []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasPrefix(name, prefix) && strings.HasSuffix(name, ".md") {
			candidates = append(candidates, name)
		}
	}

	if len(candidates) == 0 {
		return "", fmt.Errorf("no previous report found with prefix %q in %s", prefix, outDir)
	}

	// The timestamp layout (2006-01-02-150405) sorts correctly as a plain string.
	sort.Strings(candidates)
	return filepath.Join(outDir, candidates[len(candidates)-1]), nil
}

// parseReportFile reads a benchmark report (or raw `go test -bench` output)
// from disk and extracts its benchmark results.
func parseReportFile(path string) ([]benchmarkResult, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", path, err)
	}
	return parseBenchmarkResults(string(content)), nil
}

// aggregateByName collapses repeated entries for the same benchmark name
// (produced by `go test -count N` with N>1) into a single entry per name,
// averaging ns/op, B/op and allocs/op, so the comparison shows one row per
// benchmark instead of one row per repetition.
func aggregateByName(results []benchmarkResult) []benchmarkResult {
	order := make([]string, 0, len(results))
	sums := make(map[string][3]float64, len(results))
	counts := make(map[string]int, len(results))

	for _, r := range results {
		if _, seen := counts[r.Name]; !seen {
			order = append(order, r.Name)
		}
		s := sums[r.Name]
		s[0] += parseMetric(r.NsPerOp)
		s[1] += parseMetric(r.BytesOp)
		s[2] += parseMetric(r.AllocsOp)
		sums[r.Name] = s
		counts[r.Name]++
	}

	out := make([]benchmarkResult, 0, len(order))
	for _, name := range order {
		n := float64(counts[name])
		s := sums[name]
		out = append(out, benchmarkResult{
			Name:     name,
			NsPerOp:  strconv.FormatFloat(s[0]/n, 'f', -1, 64),
			BytesOp:  strconv.FormatFloat(s[1]/n, 'f', -1, 64),
			AllocsOp: strconv.FormatFloat(s[2]/n, 'f', -1, 64),
		})
	}
	return out
}

// buildComparison matches benchmarks by name across the old and new result
// sets. Benchmarks present in both come first, in the new report's order;
// benchmarks removed since the old report (present only in old) are appended
// at the end. Repeated runs of the same benchmark (-count N) are averaged
// first via aggregateByName.
func buildComparison(oldResults, newResults []benchmarkResult) []comparisonRow {
	oldResults = aggregateByName(oldResults)
	newResults = aggregateByName(newResults)

	oldByName := make(map[string]benchmarkResult, len(oldResults))
	for _, r := range oldResults {
		oldByName[r.Name] = r
	}

	seen := make(map[string]bool, len(newResults))
	rows := make([]comparisonRow, 0, len(newResults))

	for _, nr := range newResults {
		seen[nr.Name] = true
		row := comparisonRow{Name: nr.Name, HasNew: true}
		row.NewNs = parseMetric(nr.NsPerOp)
		row.NewBytes = parseMetric(nr.BytesOp)
		row.NewAllocs = parseMetric(nr.AllocsOp)
		if or, ok := oldByName[nr.Name]; ok {
			row.HasOld = true
			row.OldNs = parseMetric(or.NsPerOp)
			row.OldBytes = parseMetric(or.BytesOp)
			row.OldAllocs = parseMetric(or.AllocsOp)
		}
		rows = append(rows, row)
	}

	for _, or := range oldResults {
		if seen[or.Name] {
			continue
		}
		row := comparisonRow{Name: or.Name, HasOld: true}
		row.OldNs = parseMetric(or.NsPerOp)
		row.OldBytes = parseMetric(or.BytesOp)
		row.OldAllocs = parseMetric(or.AllocsOp)
		rows = append(rows, row)
	}

	return rows
}

func parseMetric(value string) float64 {
	f, _ := strconv.ParseFloat(value, 64)
	return f
}

// renderComparison formats rows as a markdown table under a "## Comparison:
// old → new" heading. For every metric, negative % means faster/smaller
// (improvement), positive % means slower/bigger (regression) — true for
// ns/op, B/op and allocs/op alike.
func renderComparison(oldLabel, newLabel string, rows []comparisonRow) string {
	var b strings.Builder
	fmt.Fprintf(&b, "## Comparison: %s → %s\n\n", oldLabel, newLabel)
	b.WriteString(renderComparisonTable(rows))
	return b.String()
}

// renderComparisonTable formats rows as a markdown table, without a heading,
// so callers that build their own multi-section report (see runCompareAll)
// can supply their own heading per section.
func renderComparisonTable(rows []comparisonRow) string {
	var b strings.Builder

	b.WriteString("Negative % = faster/smaller (improvement). Positive % = slower/bigger (regression).\n\n")
	b.WriteString("| Benchmark | ns/op (old → new) | Δ ns/op | B/op (old → new) | Δ B/op | allocs/op (old → new) | Δ allocs/op |\n")
	b.WriteString("| --- | ---: | ---: | ---: | ---: | ---: | ---: |\n")

	for _, row := range rows {
		fmt.Fprintf(&b, "| `%s` | %s | %s | %s | %s | %s | %s |\n",
			row.Name,
			pairString(row.HasOld, row.HasNew, row.OldNs, row.NewNs, false),
			metricDelta(row, row.OldNs, row.NewNs),
			pairString(row.HasOld, row.HasNew, row.OldBytes, row.NewBytes, false),
			metricDelta(row, row.OldBytes, row.NewBytes),
			pairString(row.HasOld, row.HasNew, row.OldAllocs, row.NewAllocs, true),
			metricDelta(row, row.OldAllocs, row.NewAllocs),
		)
	}

	return b.String()
}

func pairString(hasOld, hasNew bool, oldV, newV float64, isCount bool) string {
	return fmt.Sprintf("%s → %s", formatMetric(hasOld, oldV, isCount), formatMetric(hasNew, newV, isCount))
}

func formatMetric(has bool, v float64, isCount bool) string {
	if !has {
		return "—"
	}
	if isCount {
		return strconv.FormatInt(int64(v), 10)
	}
	return strconv.FormatFloat(v, 'f', -1, 64)
}

func metricDelta(row comparisonRow, oldV, newV float64) string {
	if !row.HasOld {
		return "novo"
	}
	if !row.HasNew {
		return "removido"
	}
	if oldV == 0 {
		return "n/a"
	}
	pct := (newV - oldV) / oldV * 100
	sign := ""
	if pct >= 0 {
		sign = "+"
	}
	return fmt.Sprintf("%s%.1f%%", sign, pct)
}
