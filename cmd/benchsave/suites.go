package main

import "strings"

// benchSuite is one named, runnable slice of the benchmark surface (one Go
// package plus a -bench regexp). benchsave is meant to be the single hub for
// running and comparing every benchmark family in the repo — to add a new
// family (e.g. a future render or ui suite), append an entry here. Nothing
// else needs to change: -suite <name> and -suite all pick it up, and its
// reports join -compare-all automatically because the report name is derived
// from the suite name.
type benchSuite struct {
	// Name is both the -suite value and the report family suffix, so a
	// suite named "runtime" produces reports named "bench-runtime-<ts>.md".
	Name      string
	Pkg       string
	Bench     string
	Benchtime string
}

var benchSuites = []benchSuite{
	{Name: "scene", Pkg: "./scene", Bench: "."},
	{Name: "runtime", Pkg: "./runtime", Bench: "."},
	{Name: "raster", Pkg: "./platform/raster", Bench: "."},
	{Name: "tess", Pkg: "./platform/wasm/tess", Bench: "BenchmarkTess.*"},
	{
		Name:      "particle",
		Pkg:       "./particle",
		Bench:     "BenchmarkEmitter(Step|Alive)$",
		Benchtime: "500ms",
	},
	{
		Name:  "runtime-seek",
		Pkg:   "./runtime",
		Bench: "BenchmarkDriver(Step|ScrubToTick)$|BenchmarkReplay$",
	},
	{
		Name:      "runtime-seek-back",
		Pkg:       "./runtime",
		Bench:     "BenchmarkDriverBack$",
		Benchtime: "500x",
	},
}

func findSuite(name string) (benchSuite, bool) {
	for _, s := range benchSuites {
		if s.Name == name {
			return s, true
		}
	}
	return benchSuite{}, false
}

func suiteNames() []string {
	names := make([]string, len(benchSuites))
	for i, s := range benchSuites {
		names[i] = s.Name
	}
	return names
}

func suiteNamesString() string {
	return strings.Join(suiteNames(), ", ")
}
