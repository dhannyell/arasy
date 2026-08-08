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
// its justification. A new finding indicates either a determinism hazard or
// a loop whose order independence must be documented.
func TestEngineSimulationPathHasNoDeterminismHazards(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	findings, err := simcheck.CheckEngine(root, "./scene/...", "./runtime/...", "./examples/...", "./internal/utilityai/...")
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 0 {
		t.Fatalf("engine self-scan reported %d finding(s):\n%s",
			len(findings), strings.Join(findings, "\n"))
	}
}

// TestCheckSkipsTheEngineOwnModule verifies that the repository's own
// render-only examples are not checked twice. Their render-only math is
// already exempted by forbidigo.
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
