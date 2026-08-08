package utilityai_test

import (
	"testing"

	"github.com/dhannyell/arasy/internal/utilityai"
)

// benchAgents is the v1 battle scale: 20k agents × 5 options × 4
// considerations = 400k curve evaluations per batch. The no-trace result
// is the regression baseline; it is also the ruler for the battle's
// time-slicing decision.
const benchAgents = 20_000

func benchmarkEvaluateBatch(b *testing.B, trace *utilityai.Trace) {
	p, inputs, prev := batchFixture(benchAgents)
	out := make([]utilityai.OptionID, benchAgents)
	b.ReportAllocs()
	for b.Loop() {
		p.EvaluateBatch(inputs, prev, out, trace)
	}
}

func BenchmarkEvaluateBatch(b *testing.B) { benchmarkEvaluateBatch(b, nil) }

func BenchmarkEvaluateBatchTrace(b *testing.B) {
	benchmarkEvaluateBatch(b, &utilityai.Trace{})
}
