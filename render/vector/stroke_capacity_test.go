package vector

import (
	"testing"

	"github.com/dhannyell/arasy/math2d"
)

func TestStrokeBufferGrowthCoversGeneratedGeometry(t *testing.T) {
	open := []math2d.Vec2{{}, {X: 10}, {X: 12, Y: 8}, {X: 20, Y: 10}}
	closed := []math2d.Vec2{{}, {X: 10}, {X: 10, Y: 10}, {Y: 10}}

	for _, test := range []struct {
		name   string
		points []math2d.Vec2
		closed bool
		style  StrokeStyle
	}{
		{name: "open bevel", points: open, style: StrokeStyle{Width: 2, LineJoin: JoinBevel}},
		{name: "open miter", points: open, style: StrokeStyle{Width: 2, LineJoin: JoinMiter}},
		{name: "open round", points: open, style: StrokeStyle{Width: 2, LineJoin: JoinRound, LineCap: CapRound}},
		{name: "closed bevel", points: closed, closed: true, style: StrokeStyle{Width: 2, LineJoin: JoinBevel}},
		{name: "closed miter", points: closed, closed: true, style: StrokeStyle{Width: 2, LineJoin: JoinMiter}},
		{name: "closed round", points: closed, closed: true, style: StrokeStyle{Width: 2, LineJoin: JoinRound}},
	} {
		t.Run(test.name, func(t *testing.T) {
			vertexGrowth, indexGrowth := strokeBufferGrowth(len(test.points), test.closed, test.style)
			var vertices []math2d.Vec2
			var indices []uint32
			appendStrokeContour(&vertices, &indices, test.points, test.closed, test.style)

			if len(vertices) > vertexGrowth || len(indices) > indexGrowth {
				t.Fatalf(
					"growth (%d vertices, %d indices) did not cover output (%d vertices, %d indices)",
					vertexGrowth,
					indexGrowth,
					len(vertices),
					len(indices),
				)
			}
		})
	}
}

func TestTrimContourFullRangeReusesFlattenedPoints(t *testing.T) {
	points := []math2d.Vec2{{}, {X: 10}, {X: 20}}
	trimmed, closed := trimContour(points, true, 0, 1)

	if !closed {
		t.Fatal("trimContour() dropped closed state")
	}
	if &trimmed[0] != &points[0] {
		t.Fatal("trimContour() copied an unchanged full-range contour")
	}
}
