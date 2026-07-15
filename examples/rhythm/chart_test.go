package rhythm

import "testing"

func TestChartIDsMatchIndexes(t *testing.T) {
	if len(chart) > maxObjects {
		t.Fatalf("chart has %d objects, maxObjects = %d", len(chart), maxObjects)
	}
	for i, object := range chart {
		if object.ID != i {
			t.Fatalf("chart[%d].ID = %d, want %d", i, object.ID, i)
		}
		if object.Kind != objectCircle && !(object.Kind == objectSpecial && object.Lane < 0) && (object.Lane < 0 || object.Lane >= laneCount) {
			t.Fatalf("chart[%d].Lane = %d, want [0, %d)", i, object.Lane, laneCount)
		}
	}
}

func TestRenderEndMSKeepsHoldVisibleThroughRelease(t *testing.T) {
	tap := chartObject{Kind: objectTap, TimeMS: 1000}
	hold := chartObject{Kind: objectHold, TimeMS: 1000, EndMS: 1600}

	if got := renderEndMS(tap); got != 1000 {
		t.Fatalf("tap render end = %d, want 1000", got)
	}
	if got := renderEndMS(hold); got != 1600 {
		t.Fatalf("hold render end = %d, want release time 1600", got)
	}
}
