package simcheck_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/dhannyell/arasy/internal/simcheck"
)

// TestAnalyzerGuardsOnlySimulationImporters checks transitive simulation
// imports, the //arasy:render opt-out, and packages outside that graph.
func TestAnalyzerGuardsOnlySimulationImporters(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), simcheck.New("simroot"),
		"simgame", "rendergame", "pure")
}
