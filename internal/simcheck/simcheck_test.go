package simcheck_test

import (
	"path/filepath"
	"strings"
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

// TestEngineSimulationPathHasNoDeterminismHazards self-scans the engine's
// simulation packages for the hazards forbidigo cannot express: map
// iteration, randomness outside ctx.Rand, and wall-clock reads. Every
// legitimate map loop carries an //arasy:order-insensitive directive with
// its justification; a new finding here is either a determinism bug or a
// loop that needs that judgment call.
func TestEngineSimulationPathHasNoDeterminismHazards(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	findings, err := simcheck.CheckEngine(root, "./scene/...", "./runtime/...", "./examples/...")
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 0 {
		t.Fatalf("engine self-scan reported %d finding(s):\n%s",
			len(findings), strings.Join(findings, "\n"))
	}
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
