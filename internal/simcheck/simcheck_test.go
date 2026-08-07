package simcheck_test

import (
	"path/filepath"
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

// TestCheckSkipsTheEngineOwnModule pins the regression where arasy build
// refused the repository's bundled examples: their render-only math is
// exempted by forbidigo, the repository's own guard, so the blessed-path
// check must not re-judge it with a second exemption map.
func TestCheckSkipsTheEngineOwnModule(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	findings, err := simcheck.Check(root, "./examples/football/native")
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 0 {
		t.Fatalf("Check on the engine module reported %d finding(s), want none:\n%s",
			len(findings), findings[0])
	}
}
