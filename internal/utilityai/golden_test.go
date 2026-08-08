package utilityai_test

import (
	"strings"
	"testing"

	"github.com/dhannyell/arasy/internal/utilityai"
)

// goldenDecisions is the expected option per agent, one digit each, for
// batchFixture(128). Generated on windows/amd64, 2026-08-08.
const goldenDecisions = "" +
	"1110002003322441100020003324411100000033224411100200032244110002" +
	"0003322441100020033224411100200033224111000200332244110002000332"

// TestGoldenDecisions pins the full decision vector. CI runs this on four
// architectures, so any transcendental drift or FMA fusion breaks it
// there. A local failure means the evaluation semantics changed: update
// the golden only for a deliberate semantic change, never for drift.
func TestGoldenDecisions(t *testing.T) {
	p, inputs, prev := batchFixture(128)
	out := make([]utilityai.OptionID, len(prev))
	p.EvaluateBatch(inputs, prev, out, nil)

	var b strings.Builder
	for _, o := range out {
		b.WriteByte('0' + byte(o))
	}
	got := b.String()
	if got != goldenDecisions {
		for i := range got {
			if got[i] != goldenDecisions[i] {
				t.Fatalf("first divergence at agent %d: got %c, want %c\nfull: %s",
					i, got[i], goldenDecisions[i], got)
			}
		}
	}
}
