# Benchmarking Guide

This directory contains benchmark baselines and reports for frameforge's performance-critical code paths.

## Structure

- **`baselines/`** — Committed baseline benchmarks (`bench-<family>.txt`), one per benchmark family. These are the reference points against which performance regressions are measured.
- **`runs/`** — Ad-hoc benchmark runs (ignored by git). Use this to organize temporary results during investigation.

## Workflow

### Running a Benchmark

Run a named suite:

```bash
go run ./cmd/benchsave -suite scene
```

List all available suites:

```bash
go run ./cmd/benchsave -list-suites
```

Run with a specific profile:

- `-profile quick` — Single run, quick feedback (count=1)
- `-profile ci` — CI-friendly (count=3, benchtime=1s)
- `-profile decision` — Statistically reliable (count=10, benchtime=2s) — **recommended for decision-making**

Example:

```bash
go run ./cmd/benchsave -suite runtime -profile decision
```

### Comparing Against Baseline

After running a benchmark, compare the results against the baseline:

```bash
go run ./cmd/benchsave -suite scene -compare baseline
```

This invokes [benchstat](https://pkg.go.dev/golang.org/x/perf/cmd/benchstat) to compute delta and statistical significance.

Output:
- `~` — No significant change (within noise margin)
- `+NN%` / `-NN%` — Measured change with statistical confidence

### Updating the Baseline

When a performance change is intentional (e.g., optimization accepted or regression fixed), promote the current run to be the new baseline:

```bash
go run ./cmd/benchsave -suite scene -profile decision -update-baseline
```

This saves the raw benchmark output to `baselines/bench-scene.txt`, overwriting any previous baseline.

### Interpreting Results

- **`~` (tilde)** — The difference is not statistically significant; treat as noise.
- **`+5%`** — 5% regression, measured with 95%+ confidence.
- **`-3%`** — 3% improvement, measured with 95%+ confidence.
- **Too few samples?** — When `-count` is low (< 6), a warning is emitted. Use `-profile decision` for reliable comparisons.

### Recommended Count Values

For **quick feedback** (during development):
- `-count 1` or `-count 3` — Fast, but noisy

For **decision-making** (accept/reject a change):
- `-count 10` or `-count 20` — Statistically reliable

Example: Accept a change only if `-profile decision` shows `~` (no regression) or `+1%` (tiny regression within acceptable threshold).

## Implementation Details

- All baselines use the same benchmark format as `go test -bench` output (benchstat-compatible).
- Baselines include metadata: commit SHA, Go version, CPU info.
- Baseline comparisons use [golang.org/x/perf/cmd/benchstat](https://pkg.go.dev/golang.org/x/perf/cmd/benchstat) for statistical rigor.
- Dependency on `golang.org/x/perf` is test-only (never shipped with the engine).

## Troubleshooting

**"no baseline for family X"** — Run with `-update-baseline` first to create it:
```bash
go run ./cmd/benchsave -suite runtime -profile decision -update-baseline
```

**Results seem inconsistent across runs** — Use `-profile decision` to increase sample count and benchtime.

**Benchmarks timing out** — Try `-timeout 10m` or `-benchtime 500ms` to adjust.
