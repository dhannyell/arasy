package utilityai_test

import (
	"testing"

	"github.com/dhannyell/arasy/internal/utilityai"
)

// batchFixture builds a deterministic batch: 5 options × 4 considerations
// over 4 input columns, every curve kind in use, commit bonus on, and a
// prev mix that includes NoOption. All inputs derive from integer
// arithmetic on the agent index, so the fixture is identical on every
// architecture.
func batchFixture(agents int) (*utilityai.Profile, utilityai.Inputs, []utilityai.OptionID) {
	const columns = 4
	curve := func(k utilityai.CurveKind, m, b, c float64) utilityai.Curve {
		return utilityai.Curve{Kind: k, M: m, B: b, C: c}
	}
	con := func(in utilityai.InputID, cv utilityai.Curve) utilityai.Consideration {
		return utilityai.Consideration{Input: in, Curve: cv}
	}
	p := &utilityai.Profile{
		CommitBonus: 0.1,
		Options: []utilityai.Option{
			{Name: "chase", Weight: 1, Considerations: []utilityai.Consideration{
				con(0, curve(utilityai.CurveLinear, 1, 0, 0)),
				con(1, curve(utilityai.CurveQuadratic, 1, 0, 0)),
				con(2, curve(utilityai.CurveLogistic, 8, 0, 0.5)),
				con(3, curve(utilityai.CurveStep, 0, 0, 0.25)),
			}},
			{Name: "flee", Weight: 0.9, Considerations: []utilityai.Consideration{
				con(0, curve(utilityai.CurveLinear, -1, 1, 0)),
				con(1, curve(utilityai.CurveCubic, 1, 0, 0)),
				con(2, curve(utilityai.CurveSmoothStep, 1, 0, 0)),
				con(3, curve(utilityai.CurveLogistic, -6, 1, 0.5)),
			}},
			{Name: "hold", Weight: 0.8, Considerations: []utilityai.Consideration{
				con(0, curve(utilityai.CurveSmoothStep, 1, 0, 0)),
				con(1, curve(utilityai.CurveStep, 0, 0, 0.5)),
				con(2, curve(utilityai.CurveQuadratic, -1, 1, 0)),
				con(3, curve(utilityai.CurveLinear, 0.5, 0.25, 0)),
			}},
			{Name: "escort", Weight: 0.7, Considerations: []utilityai.Consideration{
				con(0, curve(utilityai.CurveCubic, -1, 1, 0)),
				con(1, curve(utilityai.CurveLogistic, 12, 0, 0.75)),
				con(2, curve(utilityai.CurveLinear, 1, 0, 0)),
				con(3, curve(utilityai.CurveSmoothStep, -1, 1, 0)),
			}},
			{Name: "wander", Weight: 0.6, Considerations: []utilityai.Consideration{
				con(0, curve(utilityai.CurveStep, 0, 0, 0.75)),
				con(1, curve(utilityai.CurveLinear, 0.25, 0.5, 0)),
				con(2, curve(utilityai.CurveCubic, 0.5, 0.25, 0)),
				con(3, curve(utilityai.CurveQuadratic, 0.75, 0.125, 0)),
			}},
		},
	}
	inputs := make(utilityai.Inputs, columns)
	for c := range inputs {
		col := make([]float64, agents)
		for i := range col {
			col[i] = float64((i*7+c*13)%101) / 100
		}
		inputs[c] = col
	}
	prev := make([]utilityai.OptionID, agents)
	for i := range prev {
		if i%7 == 0 {
			prev[i] = utilityai.NoOption
		} else {
			prev[i] = utilityai.OptionID(i % len(p.Options))
		}
	}
	return p, inputs, prev
}

// TestBatchMatchesOne proves the column path: EvaluateBatch and the
// row-based evaluateOne are independent implementations, and every
// decision and every trace score must match bit for bit.
func TestBatchMatchesOne(t *testing.T) {
	p, inputs, prev := batchFixture(1000)
	agents := len(inputs[0])

	out := make([]utilityai.OptionID, agents)
	trace := &utilityai.Trace{}
	p.EvaluateBatch(inputs, prev, out, trace)

	row := make([]float64, len(inputs))
	scores := make([]float64, len(p.Options))
	for i := 0; i < agents; i++ {
		for c := range inputs {
			row[c] = inputs[c][i]
		}
		want := p.EvaluateOne(row, prev[i], scores)
		if out[i] != want {
			t.Fatalf("agent %d: batch chose %d, reference chose %d", i, out[i], want)
		}
		for oi, s := range scores {
			if trace.Scores[i][oi] != s {
				t.Fatalf("agent %d option %d: trace score %v, reference %v", i, oi, trace.Scores[i][oi], s)
			}
		}
	}
}

// TestBatchNilPrev covers the commit-free path: nil prev must equal an
// explicit all-NoOption prev.
func TestBatchNilPrev(t *testing.T) {
	p, inputs, _ := batchFixture(1000)
	agents := len(inputs[0])

	noPrev := make([]utilityai.OptionID, agents)
	for i := range noPrev {
		noPrev[i] = utilityai.NoOption
	}
	a := make([]utilityai.OptionID, agents)
	b := make([]utilityai.OptionID, agents)
	p.EvaluateBatch(inputs, nil, a, nil)
	p.EvaluateBatch(inputs, noPrev, b, nil)
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("agent %d: nil prev %d, explicit NoOption %d", i, a[i], b[i])
		}
	}
}

func identity(input utilityai.InputID) utilityai.Consideration {
	return utilityai.Consideration{
		Input: input,
		Curve: utilityai.Curve{Kind: utilityai.CurveLinear, M: 1},
	}
}

// TestEvaluateOne pins the reference semantics: dominant option, tie by
// lowest index, commit bonus, and the compensation factor.
func TestEvaluateOne(t *testing.T) {
	t.Run("dominant option wins", func(t *testing.T) {
		p := &utilityai.Profile{Options: []utilityai.Option{
			{Name: "a", Weight: 1, Considerations: []utilityai.Consideration{identity(0)}},
			{Name: "b", Weight: 1, Considerations: []utilityai.Consideration{identity(1)}},
		}}
		if got := p.EvaluateOne([]float64{0.9, 0.3}, utilityai.NoOption, nil); got != 0 {
			t.Fatalf("inputs favor option 0, got %d", got)
		}
		if got := p.EvaluateOne([]float64{0.3, 0.9}, utilityai.NoOption, nil); got != 1 {
			t.Fatalf("inputs favor option 1, got %d", got)
		}
	})

	t.Run("exact tie keeps the lowest index", func(t *testing.T) {
		same := utilityai.Option{Weight: 1, Considerations: []utilityai.Consideration{identity(0)}}
		p := &utilityai.Profile{Options: []utilityai.Option{same, same, same}}
		if got := p.EvaluateOne([]float64{0.7}, utilityai.NoOption, nil); got != 0 {
			t.Fatalf("tie must keep index 0, got %d", got)
		}
	})

	t.Run("commit bonus holds the previous option", func(t *testing.T) {
		p := &utilityai.Profile{
			CommitBonus: 0.05,
			Options: []utilityai.Option{
				{Name: "hold", Weight: 1, Considerations: []utilityai.Consideration{identity(0)}},
				{Name: "rival", Weight: 1, Considerations: []utilityai.Consideration{identity(1)}},
			},
		}
		// The rival scores slightly higher, so it wins without a previous
		// option; the bonus (0.8*1.05 = 0.84 > 0.81) holds option 0.
		inputs := []float64{0.8, 0.81}
		if got := p.EvaluateOne(inputs, utilityai.NoOption, nil); got != 1 {
			t.Fatalf("without prev the rival wins, got %d", got)
		}
		if got := p.EvaluateOne(inputs, 0, nil); got != 0 {
			t.Fatalf("with prev=0 the bonus must hold option 0, got %d", got)
		}
	})

	t.Run("compensation lifts many considerations", func(t *testing.T) {
		p := &utilityai.Profile{Options: []utilityai.Option{
			{Name: "many", Weight: 1, Considerations: []utilityai.Consideration{
				identity(0), identity(0), identity(0), identity(0),
			}},
			{Name: "single", Weight: 1, Considerations: []utilityai.Consideration{identity(1)}},
		}}
		// Raw product 0.9^4 = 0.6561 loses to 0.7; the compensated product
		// (~0.876) must win, or the factor is not applied.
		if got := p.EvaluateOne([]float64{0.9, 0.7}, utilityai.NoOption, nil); got != 0 {
			t.Fatalf("compensation must lift option 0, got %d", got)
		}
	})
}
