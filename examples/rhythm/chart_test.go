package rhythm

import "testing"

func TestChartIDsMatchIndexes(t *testing.T) {
	for i, object := range chart {
		if object.ID != i {
			t.Fatalf("chart[%d].ID = %d, want %d", i, object.ID, i)
		}
		if object.Lane < 0 || object.Lane >= laneCount {
			t.Fatalf("chart[%d].Lane = %d, want [0, %d)", i, object.Lane, laneCount)
		}
	}
}

func TestChartHasNoNotesDuringHoldOnSameLane(t *testing.T) {
	heldUntil := [laneCount]int{}
	previousTimeMS := -1

	for i, object := range chart {
		if object.TimeMS < previousTimeMS {
			t.Fatalf("chart[%d].TimeMS = %d, want at least %d", i, object.TimeMS, previousTimeMS)
		}
		previousTimeMS = object.TimeMS

		if object.Lane < 0 || object.Lane >= laneCount {
			t.Fatalf("chart[%d].Lane = %d, want [0, %d)", i, object.Lane, laneCount)
		}
		if object.TimeMS < heldUntil[object.Lane] {
			t.Fatalf("chart[%d] starts at %dms while lane %d is held until %dms", i, object.TimeMS, object.Lane, heldUntil[object.Lane])
		}
		if isHold(object) {
			heldUntil[object.Lane] = object.EndMS
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
