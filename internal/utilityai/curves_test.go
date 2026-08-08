package utilityai_test

import (
	"testing"

	"github.com/dhannyell/arasy/internal/utilityai"
)

// TestCurves pins exact outputs per curve kind. All values are exactly
// representable, so the test compares with ==: rounding drift is a
// determinism bug, not flakiness.
func TestCurves(t *testing.T) {
	cases := []struct {
		name  string
		curve utilityai.Curve
		x     float64
		want  float64
	}{
		{"linear identity at 0", utilityai.Curve{Kind: utilityai.CurveLinear, M: 1}, 0, 0},
		{"linear identity at half", utilityai.Curve{Kind: utilityai.CurveLinear, M: 1}, 0.5, 0.5},
		{"linear identity at 1", utilityai.Curve{Kind: utilityai.CurveLinear, M: 1}, 1, 1},
		{"linear slope and shift", utilityai.Curve{Kind: utilityai.CurveLinear, M: 0.5, B: 0.25}, 0.5, 0.5},
		{"linear clamps high", utilityai.Curve{Kind: utilityai.CurveLinear, M: 2}, 0.75, 1},
		{"linear clamps low", utilityai.Curve{Kind: utilityai.CurveLinear, M: 1, B: -0.5}, 0.25, 0},

		{"quadratic at half", utilityai.Curve{Kind: utilityai.CurveQuadratic, M: 1}, 0.5, 0.25},
		{"quadratic at 1", utilityai.Curve{Kind: utilityai.CurveQuadratic, M: 1}, 1, 1},

		{"cubic at half", utilityai.Curve{Kind: utilityai.CurveCubic, M: 1}, 0.5, 0.125},
		{"cubic at 1", utilityai.Curve{Kind: utilityai.CurveCubic, M: 1}, 1, 1},

		{"smoothstep at 0", utilityai.Curve{Kind: utilityai.CurveSmoothStep, M: 1}, 0, 0},
		{"smoothstep at quarter", utilityai.Curve{Kind: utilityai.CurveSmoothStep, M: 1}, 0.25, 0.15625},
		{"smoothstep at half", utilityai.Curve{Kind: utilityai.CurveSmoothStep, M: 1}, 0.5, 0.5},
		{"smoothstep at 1", utilityai.Curve{Kind: utilityai.CurveSmoothStep, M: 1}, 1, 1},

		// At x == C the exponent is 0, Exp(0) == 1, and y == 0.5 + B exactly.
		{"logistic at midpoint", utilityai.Curve{Kind: utilityai.CurveLogistic, M: 12, C: 0.5}, 0.5, 0.5},
		{"logistic midpoint plus shift", utilityai.Curve{Kind: utilityai.CurveLogistic, M: 12, C: 0.5, B: 0.25}, 0.5, 0.75},
		{"logistic flat when M is 0", utilityai.Curve{Kind: utilityai.CurveLogistic, C: 0.5}, 0.8, 0.5},
		{"logistic clamps with shift", utilityai.Curve{Kind: utilityai.CurveLogistic, M: 12, C: 0.5, B: 0.6}, 1, 1},

		{"step below threshold", utilityai.Curve{Kind: utilityai.CurveStep, C: 0.5}, 0.25, 0},
		{"step at threshold", utilityai.Curve{Kind: utilityai.CurveStep, C: 0.5}, 0.5, 1},
		{"step above threshold", utilityai.Curve{Kind: utilityai.CurveStep, C: 0.5}, 1, 1},
		{"step at 0 with zero threshold", utilityai.Curve{Kind: utilityai.CurveStep}, 0, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.curve.Evaluate(tc.x); got != tc.want {
				t.Fatalf("Evaluate(%v) = %v, want %v", tc.x, got, tc.want)
			}
		})
	}

	t.Run("logistic is steeper with larger M", func(t *testing.T) {
		steep := utilityai.Curve{Kind: utilityai.CurveLogistic, M: 12, C: 0.5}
		soft := utilityai.Curve{Kind: utilityai.CurveLogistic, M: 4, C: 0.5}
		s, f := steep.Evaluate(0.75), soft.Evaluate(0.75)
		if !(s > f && f > 0.5 && s < 1) {
			t.Fatalf("above the midpoint: steep %v, soft %v", s, f)
		}
	})
}
