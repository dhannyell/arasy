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
	pkg := flag.String("pkg", "./scene", "package pattern passed to go test")
	bench := flag.String("bench", ".", "benchmark regexp passed to -bench")
	run := flag.String("run", "^$", "test regexp passed to -run")
	outDir := flag.String("out", "benchmarks", "directory where the benchmark report is saved")
	name := flag.String("name", "", "optional report name without extension")
	count := flag.Int("count", 1, "benchmark count passed to -count")
	benchmem := flag.Bool("benchmem", true, "include memory allocation metrics")
	flag.Parse()

	start := time.Now()
	args := []string{"test", *pkg, "-run", *run, "-bench", *bench, "-count", fmt.Sprint(*count)}
	if *benchmem {
		args = append(args, "-benchmem")
	}

	cmd := exec.Command("go", args...)

	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output

	err := cmd.Run()
	duration := time.Since(start)

	if mkdirErr := os.MkdirAll(*outDir, 0o755); mkdirErr != nil {
		fmt.Fprintf(os.Stderr, "failed to create output directory: %v\n", mkdirErr)
		os.Exit(1)
	}

	reportName := *name
	if reportName == "" {
		reportName = "bench-" + sanitizeName(*pkg) + "-" + start.Format(timestampLayout)
	}

	reportPath := filepath.Join(*outDir, reportName+".md")
	report := buildReport(start, duration, args, output.String(), err)

	if writeErr := os.WriteFile(reportPath, []byte(report), 0o644); writeErr != nil {
		fmt.Fprintf(os.Stderr, "failed to write benchmark report: %v\n", writeErr)
		os.Exit(1)
	}

	fmt.Print(output.String())
	fmt.Printf("\nbenchmark report saved to %s\n", reportPath)

	if err != nil {
		fmt.Fprintf(os.Stderr, "benchmark command failed: %v\n", err)
		os.Exit(1)
	}
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
