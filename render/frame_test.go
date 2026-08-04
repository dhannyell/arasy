package render

import (
	"testing"

	"github.com/dhannyell/arasy/math2d"
)

// FrameGround exists to replace framing numbers found by eye, so the contract
// worth pinning is the geometry: under the 2:1 view a square board projects
// to a diamond whose extremes come from three different corners, and the
// margin is a fraction of that projected size, not of the board's.
func TestFrameGroundBoxesTheProjectedBoard(t *testing.T) {
	iso := math2d.Affine{A: 1, C: -1, B: 0.5, D: 0.5}
	board := math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 80, Y: 64})

	tight := FrameGround(iso, board, 0)
	if want := (math2d.Vec2{X: -64, Y: 0}); tight.Min != want {
		t.Fatalf("tight frame Min = %v, want %v", tight.Min, want)
	}
	if want := (math2d.Vec2{X: 80, Y: 72}); tight.Max != want {
		t.Fatalf("tight frame Max = %v, want %v", tight.Max, want)
	}

	framed := FrameGround(iso, board, 0.25)
	if got, want := framed.Width(), 144*1.5; got != want {
		t.Fatalf("framed width = %v, want %v (a quarter of air per side)", got, want)
	}
	if got, want := framed.Height(), 72*1.5; got != want {
		t.Fatalf("framed height = %v, want %v", got, want)
	}
}
