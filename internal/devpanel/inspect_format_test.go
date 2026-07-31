package devpanel

import (
	"strings"
	"testing"
)

type treeVec struct {
	X, Y float64
}

type treeNested struct {
	Inner treeNestedInner
}

type treeNestedInner struct {
	InnerMost treeVec
}

type treeRoot struct {
	Name   string
	Pos    treeVec
	Nested treeNested
	Scores []int
	Table  map[string]int
	Next   *treeRoot
	hidden int
}

func joinLines(lines []string) string { return strings.Join(lines, "\n") }

func TestFormatTreeExpandsStructsAndInlinesScalarOnes(t *testing.T) {
	lines := formatTree(treeRoot{
		Name:   "root",
		Pos:    treeVec{X: 1, Y: 2},
		Nested: treeNested{Inner: treeNestedInner{InnerMost: treeVec{X: 3, Y: 4}}},
		hidden: 9,
	})
	got := joinLines(lines)

	for _, want := range []string{
		"Name: root",
		"Pos: {X:1 Y:2}", // all-scalar struct stays on one line
		"Nested: devpanel.treeNested",
		"Inner: devpanel.treeNestedInner",
		"InnerMost: {X:3 Y:4}",
		"hidden: 9", // unexported fields are still readable
		"Scores: []int [len 0]",
		"Next: nil",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("tree missing %q:\n%s", want, got)
		}
	}
}

func TestFormatTreeCapsSlices(t *testing.T) {
	lines := formatTree(struct{ Items []int }{Items: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}})
	got := joinLines(lines)

	if !strings.Contains(got, "[7]: 7") {
		t.Errorf("tree missing the eighth item:\n%s", got)
	}
	if strings.Contains(got, "[8]") {
		t.Errorf("tree showed past the item cap:\n%s", got)
	}
	if !strings.Contains(got, "+2 more") {
		t.Errorf("tree missing the remainder marker:\n%s", got)
	}
}

func TestFormatTreeSortsMapKeys(t *testing.T) {
	lines := formatTree(struct{ M map[string]int }{M: map[string]int{"b": 2, "a": 1, "c": 3}})
	got := joinLines(lines)

	a := strings.Index(got, "[a]")
	b := strings.Index(got, "[b]")
	c := strings.Index(got, "[c]")
	if a < 0 || b < 0 || c < 0 || !(a < b && b < c) {
		t.Fatalf("map keys not sorted (a=%d b=%d c=%d):\n%s", a, b, c, got)
	}
}

func TestFormatTreeStopsAtTheDepthCap(t *testing.T) {
	// A cyclic pointer chain must terminate: each level costs one depth.
	root := &treeRoot{}
	root.Next = root

	lines := formatTree(root)
	if len(lines) == 0 || !strings.Contains(joinLines(lines), "…") {
		t.Fatalf("cyclic value did not stop at the depth cap:\n%s", joinLines(lines))
	}
	if len(lines) > inspectMaxLines+1 {
		t.Fatalf("cyclic value produced %d lines, cap is %d", len(lines), inspectMaxLines)
	}
}

func TestFormatTreeCapsTotalLines(t *testing.T) {
	// Breadth over depth: five levels of eight items each beat the line cap.
	type l4 struct{ V [8]int }
	type l3 struct{ V [8]l4 }
	type l2 struct{ V [8]l3 }
	type l1 struct{ V [8]l2 }

	lines := formatTree(l1{})
	if len(lines) > inspectMaxLines+1 {
		t.Fatalf("wide value produced %d lines, cap is %d", len(lines), inspectMaxLines)
	}
	if lines[len(lines)-1] != "…" {
		t.Fatalf("capped tree does not end with the marker: %q", lines[len(lines)-1])
	}
}

func TestTruncateToWidth(t *testing.T) {
	const size = 12.0
	short := "Score: 7"
	if got := truncateToWidth(short, 400, size); got != short {
		t.Fatalf("truncateToWidth(short) = %q, want it untouched", got)
	}

	long := strings.Repeat("field name with a long value ", 20)
	got := truncateToWidth(long, 120, size)
	if !strings.HasSuffix(got, "…") {
		t.Fatalf("truncated line %q does not end with the ellipsis", got)
	}
	if len(got) >= len(long) {
		t.Fatalf("truncated line %q is not shorter than the input", got)
	}
}
