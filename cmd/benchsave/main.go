package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const timestampLayout = "2006-01-02-150405"

type benchmarkResult struct {
	Name     string
	Runs     string
	NsPerOp  string
	BytesOp  string
	AllocsOp string
	Other    []string
}

func main() {
	pkg := flag.String("pkg", "./scene", "package pattern passed to go test (ignored when -suite is set)")
	bench := flag.String("bench", ".", "benchmark regexp passed to -bench (ignored when -suite is set)")
	run := flag.String("run", "^$", "test regexp passed to -run")
	outDir := flag.String("out", "benchmarks", "directory where the benchmark report is saved")
	name := flag.String("name", "", "optional report name without extension (ignored when -suite all is set)")
	count := flag.Int("count", 1, "benchmark count passed to -count")
	benchmem := flag.Bool("benchmem", true, "include memory allocation metrics")
	benchtime := flag.String("benchtime", "", "optional benchmark time passed to -benchtime (e.g. 2s or 300x)")
	timeout := flag.String("timeout", "", "optional test timeout passed to -timeout (e.g. 30m)")
	compare := flag.String("compare", "", `after saving the new report, compare it against a previous one and print the %% change per benchmark (ns/op, B/op, allocs/op). Use "auto" to pick the most recent report for -pkg in -out, or pass a path to a specific report.`)
	diff := flag.Bool("diff", false, "skip running benchmarks; compare two existing reports instead. Usage: -diff <old-report.md> <new-report.md>")
	compareAll := flag.Bool("compare-all", false, "skip running benchmarks; compare every report family found in -out (e.g. bench-scene, bench-runtime), diffing each family's two most recent reports")
	comparisonOut := flag.String("comparison-out", "", "optional path to also save the comparison report as its own markdown file (used with -diff, -compare or -compare-all)")
	suite := flag.String("suite", "", `named benchmark suite to run instead of -pkg/-bench (see -list-suites), or "all" to run every registered suite`)
	listSuites := flag.Bool("list-suites", false, "print the registered benchmark suites and exit")
	flag.Parse()

	if *listSuites {
		fmt.Println("registered benchmark suites:")
		for _, s := range benchSuites {
			fmt.Printf("  %-12s pkg=%-12s bench=%s\n", s.Name, s.Pkg, s.Bench)
		}
		fmt.Println(`use -suite <name> to run one, or -suite all to run every suite`)
		return
	}

	if *diff {
		args := flag.Args()
		if len(args) != 2 {
			fmt.Fprintln(os.Stderr, "usage: go run ./cmd/benchsave -diff [-comparison-out <path>] <old-report.md> <new-report.md>")
			os.Exit(1)
		}
		if err := runDiffOnly(args[0], args[1], *comparisonOut); err != nil {
			fmt.Fprintf(os.Stderr, "comparison failed: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if *compareAll {
		if err := runCompareAll(*outDir, *comparisonOut); err != nil {
			fmt.Fprintf(os.Stderr, "comparison failed: %v\n", err)
			os.Exit(1)
		}
		return
	}

	baseOpts := runOptions{
		Run:           *run,
		OutDir:        *outDir,
		Count:         *count,
		Benchmem:      *benchmem,
		Benchtime:     *benchtime,
		Timeout:       *timeout,
		Compare:       *compare,
		ComparisonOut: *comparisonOut,
	}

	if *suite == "all" {
		failed := false
		for i, s := range benchSuites {
			opts := baseOpts
			opts.Pkg = s.Pkg
			opts.Bench = s.Bench
			if _, err := runOnce(opts); err != nil {
				fmt.Fprintf(os.Stderr, "suite %q failed: %v\n", s.Name, err)
				failed = true
			}
			if i < len(benchSuites)-1 {
				fmt.Println()
			}
		}
		if failed {
			os.Exit(1)
		}
		return
	}

	opts := baseOpts
	opts.Pkg = *pkg
	opts.Bench = *bench
	opts.Name = *name

	if *suite != "" {
		s, ok := findSuite(*suite)
		if !ok {
			fmt.Fprintf(os.Stderr, "unknown suite %q, available suites: %s\n", *suite, suiteNamesString())
			os.Exit(1)
		}
		opts.Pkg = s.Pkg
		opts.Bench = s.Bench
	}

	if _, err := runOnce(opts); err != nil {
		fmt.Fprintf(os.Stderr, "benchmark command failed: %v\n", err)
		os.Exit(1)
	}
}

type runOptions struct {
	Pkg, Bench, Run, OutDir, Name string
	Count                         int
	Benchmem                      bool
	Benchtime, Timeout            string
	Compare, ComparisonOut        string
}

// runOnce runs one benchmark suite (a single go test -bench invocation),
// saves its report, optionally compares it against a previous report, and
// prints the results. It is the shared implementation behind the default
// -pkg/-bench run, -suite <name>, and each iteration of -suite all.
func runOnce(opts runOptions) (reportPath string, err error) {
	start := time.Now()
	args := []string{"test", opts.Pkg, "-run", opts.Run, "-bench", opts.Bench, "-count", fmt.Sprint(opts.Count)}
	if opts.Benchmem {
		args = append(args, "-benchmem")
	}
	if opts.Benchtime != "" {
		args = append(args, "-benchtime", opts.Benchtime)
	}
	if opts.Timeout != "" {
		args = append(args, "-timeout", opts.Timeout)
	}

	cmd := exec.Command("go", args...)

	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output

	runErr := cmd.Run()
	duration := time.Since(start)

	if mkdirErr := os.MkdirAll(opts.OutDir, 0o755); mkdirErr != nil {
		return "", fmt.Errorf("failed to create output directory: %w", mkdirErr)
	}

	reportName := opts.Name
	if reportName == "" {
		reportName = "bench-" + sanitizeName(opts.Pkg) + "-" + start.Format(timestampLayout)
	}

	reportPath = filepath.Join(opts.OutDir, reportName+".md")
	report := buildReport(start, duration, args, output.String(), runErr)

	var comparison string
	if opts.Compare != "" {
		var compareErr error
		comparison, compareErr = compareAgainstPrevious(opts.Compare, opts.OutDir, opts.Pkg, reportName+".md", output.String())
		if compareErr != nil {
			fmt.Fprintf(os.Stderr, "comparison skipped: %v\n", compareErr)
			comparison = ""
		} else {
			report += "\n" + comparison
			if opts.ComparisonOut != "" {
				if writeErr := os.WriteFile(opts.ComparisonOut, []byte(comparison), 0o644); writeErr != nil {
					fmt.Fprintf(os.Stderr, "failed to write comparison report: %v\n", writeErr)
				}
			}
		}
	}

	if writeErr := os.WriteFile(reportPath, []byte(report), 0o644); writeErr != nil {
		return "", fmt.Errorf("failed to write benchmark report: %w", writeErr)
	}

	fmt.Print(output.String())
	if comparison != "" {
		fmt.Println()
		fmt.Print(comparison)
	}
	fmt.Printf("\nbenchmark report saved to %s\n", reportPath)

	if runErr != nil {
		return reportPath, fmt.Errorf("%w", runErr)
	}
	return reportPath, nil
}

func buildReport(start time.Time, duration time.Duration, args []string, output string, runErr error) string {
	status := "pass"
	if runErr != nil {
		status = "fail"
	}

	var b strings.Builder
	b.WriteString("# Benchmark Report\n\n")
	b.WriteString("## Metadata\n\n")
	b.WriteString(fmt.Sprintf("- Status: `%s`\n", status))
	b.WriteString(fmt.Sprintf("- Created at: `%s`\n", start.Format(time.RFC3339)))
	b.WriteString(fmt.Sprintf("- Duration: `%s`\n", duration.Round(time.Millisecond)))
	b.WriteString(fmt.Sprintf("- OS: `%s`\n", runtime.GOOS))
	b.WriteString(fmt.Sprintf("- Arch: `%s`\n", runtime.GOARCH))
	b.WriteString(fmt.Sprintf("- CPU count: `%d`\n", runtime.NumCPU()))
	b.WriteString(fmt.Sprintf("- Command: `%s`\n", commandString(args)))
	if runErr != nil {
		b.WriteString(fmt.Sprintf("- Error: `%v`\n", runErr))
	}

	results := parseBenchmarkResults(output)
	if len(results) > 0 {
		b.WriteString("\n## Results\n\n")
		b.WriteString("| Benchmark | Runs | ns/op | B/op | allocs/op |\n")
		b.WriteString("| --- | ---: | ---: | ---: | ---: |\n")
		for _, result := range results {
			b.WriteString(fmt.Sprintf(
				"| `%s` | %s | %s | %s | %s |\n",
				result.Name,
				tableValue(result.Runs),
				tableValue(result.NsPerOp),
				tableValue(result.BytesOp),
				tableValue(result.AllocsOp),
			))
		}
	}

	b.WriteString("\n## Raw Output\n\n")
	b.WriteString("```text\n")
	b.WriteString(output)
	if output != "" && !strings.HasSuffix(output, "\n") {
		b.WriteString("\n")
	}
	b.WriteString("```\n")

	return b.String()
}

func parseBenchmarkResults(output string) []benchmarkResult {
	var results []benchmarkResult

	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "Benchmark") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		result := benchmarkResult{
			Name: fields[0],
			Runs: fields[1],
		}

		for i := 2; i < len(fields); i++ {
			if i+1 >= len(fields) {
				result.Other = append(result.Other, fields[i])
				continue
			}

			value := fields[i]
			unit := fields[i+1]

			switch unit {
			case "ns/op":
				result.NsPerOp = value
				i++
			case "B/op":
				result.BytesOp = value
				i++
			case "allocs/op":
				result.AllocsOp = value
				i++
			default:
				result.Other = append(result.Other, fields[i])
			}
		}

		results = append(results, result)
	}

	return results
}

func tableValue(value string) string {
	if value == "" {
		return "-"
	}

	return value
}

func commandString(args []string) string {
	parts := append([]string{"go"}, args...)
	return strings.Join(parts, " ")
}

func sanitizeName(value string) string {
	value = strings.TrimPrefix(value, "./")
	value = strings.Trim(value, `/\.`)
	if value == "" {
		return "all"
	}

	replacer := strings.NewReplacer(
		"/", "-",
		"\\", "-",
		".", "-",
		"*", "all",
		":", "-",
	)

	return replacer.Replace(value)
}
