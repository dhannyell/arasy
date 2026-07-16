package main

import "testing"

func TestParticleSuiteTargetsEmitterOccupancyBenchmarks(t *testing.T) {
	suite, ok := findSuite("particle")
	if !ok {
		t.Fatal("findSuite(particle) = false, want registered suite")
	}

	if suite.Pkg != "./particle" {
		t.Fatalf("particle suite package = %q, want ./particle", suite.Pkg)
	}
	if suite.Bench != "BenchmarkEmitter(Step|Alive)$" {
		t.Fatalf("particle suite benchmark = %q", suite.Bench)
	}
	if suite.Benchtime != "500ms" {
		t.Fatalf("particle suite benchtime = %q, want 500ms", suite.Benchtime)
	}
}
