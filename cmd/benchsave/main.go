package main

import (
	"bytes"
	"encoding/json"
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
const defaultOutDir = "benchmarks/runs"

type benchmarkResult struct {
	Name     string   `json:"name"`
	Runs     string   `json:"runs"`
	NsPerOp  string   `json:"ns_per_op,omitempty"`
	BytesOp  string   `json:"bytes_op,omitempty"`
	AllocsOp string   `json:"allocs_op,omitempty"`
	Other    []string `json:"other,omitempty"`
}

type benchmarkReport struct {
	Status     string            `json:"status"`
	CreatedAt  string            `json:"created_at"`
	Duration   string            `json:"duration"`
	OS         string            `json:"os"`
	Arch       string            `json:"arch"`
	CPUCount   int               `json:"cpu_count"`
	CommitSHA  string            `json:"commit_sha,omitempty"`
	GoVersion  string            `json:"go_version,omitempty"`
	GOMAXPROCS int               `json:"gomaxprocs,omitempty"`
	Command    string            `json:"command"`
	Error      string            `json:"error,omitempty"`
	Results    []benchmarkResult `json:"results"`
	RawOutput  string            `json:"raw_output"`
}

func main() {
	pkg := flag.String("pkg", "./scene", "package pattern passed to go test (ignored when -suite is set)")
	bench := flag.String("bench", ".", "benchmark regexp passed to -bench (ignored when -suite is set)")
	run := flag.String("run", "^$", "test regexp passed to -run")
	outDir := flag.String("out", defaultOutDir, "directory where the benchmark report is saved")
	name := flag.String("name", "", "optional report name without extension (ignored when -suite all is set)")
	profile := flag.String("profile", "", `benchmark run profile: "quick" (count=1), "decision" (count=5, benchtime=2s), or "ci" (count=3, benchtime=1s). Explicit -count/-benchtime values still win.`)
	count := flag.Int("count", 1, "benchmark count passed to -count")
	benchmem := flag.Bool("benchmem", true, "include memory allocation metrics")
	benchtime := flag.String("benchtime", "", "optional benchmark time passed to -benchtime (e.g. 2s or 300x)")
	timeout := flag.String("timeout", "", "optional test timeout passed to -timeout (e.g. 30m)")
	compare := flag.String("compare", "", `after saving the new report, compare it against a previous one and print the %% change per benchmark (ns/op, B/op, allocs/op). Use "auto" to pick the most recent report for -pkg in -out, "baseline" to use the explicit baseline for this family, or pass a path to a specific report.`)
	diff := flag.Bool("diff", false, "skip running benchmarks; compare two existing reports instead. Usage: -diff <old-report.md> <new-report.md>")
	compareAll := flag.Bool("compare-all", false, "skip running benchmarks; compare every report family found in -out (e.g. bench-scene, bench-runtime), diffing each family's two most recent reports")
	comparisonOut := flag.String("comparison-out", "", "optional path to also save the comparison report as its own markdown file (used with -diff, -compare or -compare-all)")
	updateBaseline := flag.Bool("update-baseline", false, "save the current benchmark run as the baseline for this family (requires -suite or -pkg)")
	jsonOut := flag.Bool("json", true, "also save a machine-readable JSON report next to each Markdown report")
	suite := flag.String("suite", "", `named benchmark suite to run instead of -pkg/-bench (see -list-suites), or "all" to run every registered suite`)
	listSuites := flag.Bool("list-suites", false, "print the registered benchmark suites and exit")
	flag.Parse()
	countSet := flagWasSet("count")
	benchtimeSet := flagWasSet("benchtime")

	if *listSuites {
		fmt.Println("registered benchmark suites:")
		for _, s := range benchSuites {
			benchtime := s.Benchtime
			if benchtime == "" {
				benchtime = "-"
			}
			fmt.Printf("  %-18s pkg=%-12s bench=%-48s benchtime=%s\n", s.Name, s.Pkg, s.Bench, benchtime)
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

	selectedProfile, err := applyRunProfile(*profile, *count, countSet, *benchtime, benchtimeSet)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	baseOpts := runOptions{
		Run:            *run,
		OutDir:         *outDir,
		Count:          selectedProfile.Count,
		Benchmem:       *benchmem,
		Benchtime:      selectedProfile.Benchtime,
		Timeout:        *timeout,
		Compare:        *compare,
		ComparisonOut:  *comparisonOut,
		UpdateBaseline: *updateBaseline,
		JSON:           *jsonOut,
	}

	if *suite == "all" {
		failed := false
		for i, s := range benchSuites {
			opts := baseOpts
			opts.Pkg = s.Pkg
			opts.Bench = s.Bench
			opts.Family = s.Name
			if opts.Benchtime == "" {
				opts.Benchtime = s.Benchtime
			}
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
		opts.Family = s.Name
		if opts.Benchtime == "" {
			opts.Benchtime = s.Benchtime
		}
	}

	if _, err := runOnce(opts); err != nil {
		fmt.Fprintf(os.Stderr, "benchmark command failed: %v\n", err)
		os.Exit(1)
	}
}

type runOptions struct {
	Pkg, Bench, Run, OutDir, Name, Family string
	Count                                 int
	Benchmem                              bool
	Benchtime, Timeout                    string
	Compare, ComparisonOut                string
	UpdateBaseline                        bool
	JSON                                  bool
}

type runProfile struct {
	Count     int
	Benchtime string
}

func flagWasSet(name string) bool {
	wasSet := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == name {
			wasSet = true
		}
	})
	return wasSet
}

func applyRunProfile(profile string, count int, countSet bool, benchtime string, benchtimeSet bool) (runProfile, error) {
	opts := runProfile{Count: count, Benchtime: benchtime}
	if profile == "" {
		return opts, nil
	}

	switch profile {
	case "quick":
	case "ci":
		if !countSet {
			opts.Count = 3
		}
		if !benchtimeSet {
			opts.Benchtime = "1s"
		}
	case "decision":
		if !countSet {
			opts.Count = 10
		}
		if !benchtimeSet {
			opts.Benchtime = "2s"
		}
	default:
		return runProfile{}, fmt.Errorf("unknown -profile %q; use quick, ci, or decision", profile)
	}

	return opts, nil
}

// runOnce runs one benchmark suite (a single go test -bench invocation),
// saves its report, optionally compares it against a previous report, and
// prints the results. It is the shared implementation behind the default
// -pkg/-bench run, -suite <name>, and each iteration of -suite all.
func runOnce(opts runOptions) (reportPath string, err error) {
	if opts.Compare != "" && opts.Count < 6 {
		fmt.Fprintf(os.Stderr, "warning: -count=%d is too low for statistical significance (recommend >= 6); results may be unreliable\n", opts.Count)
	}

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
		family := opts.Family
		if family == "" {
			family = opts.Pkg
		}
		reportName = "bench-" + sanitizeName(family) + "-" + start.Format(timestampLayout)
	}

	reportPath = filepath.Join(opts.OutDir, reportName+".md")
	reportData := newBenchmarkReport(start, duration, args, output.String(), runErr)
	report := renderBenchmarkReport(reportData)

	var comparison string
	if opts.Compare != "" {
		family := opts.Family
		if family == "" {
			family = opts.Pkg
		}

		compareFlag := opts.Compare
		if compareFlag == "baseline" {
			baselinePath, err := resolveBaselinePath(baselineRootDir(opts.OutDir), family)
			if err != nil {
				fmt.Fprintf(os.Stderr, "comparison skipped: %v\n", err)
				compareFlag = ""
			} else {
				compareFlag = baselinePath
			}
		}

		if compareFlag != "" {
			var compareErr error
			comparison, compareErr = compareAgainstPrevious(compareFlag, opts.OutDir, family, reportName+".md", output.String())
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
	}

	if writeErr := os.WriteFile(reportPath, []byte(report), 0o644); writeErr != nil {
		return "", fmt.Errorf("failed to write benchmark report: %w", writeErr)
	}
	if opts.JSON {
		jsonPath := filepath.Join(opts.OutDir, reportName+".json")
		if writeErr := writeJSONReport(jsonPath, reportData); writeErr != nil {
			return "", writeErr
		}
	}

	fmt.Print(output.String())
	if comparison != "" {
		fmt.Println()
		fmt.Print(comparison)
	}
	fmt.Printf("\nbenchmark report saved to %s\n", reportPath)

	if opts.UpdateBaseline {
		family := opts.Family
		if family == "" {
			family = opts.Pkg
		}
		if err := saveBaseline(baselineRootDir(opts.OutDir), family, output.String()); err != nil {
			fmt.Fprintf(os.Stderr, "failed to update baseline: %v\n", err)
			if runErr != nil {
				return reportPath, fmt.Errorf("%w", runErr)
			}
			return reportPath, err
		}
		baselinePath := filepath.Join(baselineRootDir(opts.OutDir), baselineDir, "bench-"+sanitizeName(family)+".txt")
		fmt.Printf("baseline updated: %s\n", baselinePath)
	}

	if runErr != nil {
		return reportPath, fmt.Errorf("%w", runErr)
	}
	return reportPath, nil
}

func newBenchmarkReport(start time.Time, duration time.Duration, args []string, output string, runErr error) benchmarkReport {
	status := "pass"
	errText := ""
	if runErr != nil {
		status = "fail"
		errText = runErr.Error()
	}

	return benchmarkReport{
		Status:     status,
		CreatedAt:  start.Format(time.RFC3339),
		Duration:   duration.Round(time.Millisecond).String(),
		OS:         runtime.GOOS,
		Arch:       runtime.GOARCH,
		CPUCount:   runtime.NumCPU(),
		CommitSHA:  getCommitSHA(),
		GoVersion:  runtime.Version(),
		GOMAXPROCS: runtime.GOMAXPROCS(-1),
		Command:    commandString(args),
		Error:      errText,
		Results:    parseBenchmarkResults(output),
		RawOutput:  output,
	}
}

func getCommitSHA() string {
	cmd := exec.Command("git", "rev-parse", "--short", "HEAD")
	cmd.Stderr = nil
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func renderBenchmarkReport(report benchmarkReport) string {
	var b strings.Builder
	b.WriteString("# Benchmark Report\n\n")
	b.WriteString("## Metadata\n\n")
	b.WriteString(fmt.Sprintf("- Status: `%s`\n", report.Status))
	b.WriteString(fmt.Sprintf("- Created at: `%s`\n", report.CreatedAt))
	b.WriteString(fmt.Sprintf("- Duration: `%s`\n", report.Duration))
	b.WriteString(fmt.Sprintf("- OS: `%s`\n", report.OS))
	b.WriteString(fmt.Sprintf("- Arch: `%s`\n", report.Arch))
	b.WriteString(fmt.Sprintf("- CPU count: `%d`\n", report.CPUCount))
	if report.CommitSHA != "" {
		b.WriteString(fmt.Sprintf("- Commit SHA: `%s`\n", report.CommitSHA))
	}
	if report.GoVersion != "" {
		b.WriteString(fmt.Sprintf("- Go version: `%s`\n", report.GoVersion))
	}
	if report.GOMAXPROCS > 0 {
		b.WriteString(fmt.Sprintf("- GOMAXPROCS: `%d`\n", report.GOMAXPROCS))
	}
	b.WriteString(fmt.Sprintf("- Command: `%s`\n", report.Command))
	if report.Error != "" {
		b.WriteString(fmt.Sprintf("- Error: `%s`\n", report.Error))
	}

	if len(report.Results) > 0 {
		b.WriteString("\n## Results\n\n")
		b.WriteString("| Benchmark | Runs | ns/op | B/op | allocs/op |\n")
		b.WriteString("| --- | ---: | ---: | ---: | ---: |\n")
		for _, result := range report.Results {
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
	b.WriteString(report.RawOutput)
	if report.RawOutput != "" && !strings.HasSuffix(report.RawOutput, "\n") {
		b.WriteString("\n")
	}
	b.WriteString("```\n")

	return b.String()
}

func writeJSONReport(path string, report benchmarkReport) error {
	content, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal benchmark JSON report: %w", err)
	}
	content = append(content, '\n')
	if err := os.WriteFile(path, content, 0o644); err != nil {
		return fmt.Errorf("failed to write benchmark JSON report: %w", err)
	}
	return nil
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
