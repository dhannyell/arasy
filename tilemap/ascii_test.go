package tilemap

import (
	"strings"
	"testing"
)

func TestParseASCIIUsesRuneLegendAndRowMajorOrder(t *testing.T) {
	rows := []string{
		"MMm....~~~",
		"Mm..f..~~~",
		"....f...~~",
		"..B.....~~",
		"....BLf...",
		".f....B...",
		"...f......",
		".f....f...",
	}
	legend := map[rune]int16{
		'M': 0,
		'm': 1,
		'~': 2,
		'f': 3,
		'B': 4,
		'L': 5,
		'.': -1,
	}

	layer, err := ParseASCII(rows, legend)
	if err != nil {
		t.Fatalf("ParseASCII: %v", err)
	}
	if layer.W != 10 || layer.H != 8 {
		t.Fatalf("layer size = %dx%d, want 10x8", layer.W, layer.H)
	}
	want := []int16{
		0, 0, 1, -1, -1, -1, -1, 2, 2, 2,
		0, 1, -1, -1, 3, -1, -1, 2, 2, 2,
		-1, -1, -1, -1, 3, -1, -1, -1, 2, 2,
		-1, -1, 4, -1, -1, -1, -1, -1, 2, 2,
		-1, -1, -1, -1, 4, 5, 3, -1, -1, -1,
		-1, 3, -1, -1, -1, -1, 4, -1, -1, -1,
		-1, -1, -1, 3, -1, -1, -1, -1, -1, -1,
		-1, 3, -1, -1, -1, -1, 3, -1, -1, -1,
	}
	if len(layer.Tiles) != len(want) {
		t.Fatalf("tile count = %d, want %d", len(layer.Tiles), len(want))
	}
	for i := range want {
		if layer.Tiles[i] != want[i] {
			t.Errorf("tile[%d] = %d, want %d", i, layer.Tiles[i], want[i])
		}
	}
}

func TestParseASCIIRejectsUnequalRowWidths(t *testing.T) {
	_, err := ParseASCII([]string{"..", "."}, map[rune]int16{'.': -1})
	if err == nil || !strings.Contains(err.Error(), "row 1") || !strings.Contains(err.Error(), "width 1") {
		t.Fatalf("unequal rows error = %v", err)
	}
}

func TestParseASCIIReportsUnknownRunePosition(t *testing.T) {
	_, err := ParseASCII([]string{"..", ".x"}, map[rune]int16{'.': -1})
	if err == nil || !strings.Contains(err.Error(), "unknown rune 'x'") ||
		!strings.Contains(err.Error(), "row 1") || !strings.Contains(err.Error(), "column 1") {
		t.Fatalf("unknown rune error = %v", err)
	}
}

func TestParseASCIIAllowsEmptyInput(t *testing.T) {
	layer, err := ParseASCII(nil, nil)
	if err != nil {
		t.Fatalf("ParseASCII empty input: %v", err)
	}
	if layer.W != 0 || layer.H != 0 || layer.Tiles != nil {
		t.Fatalf("empty layer = %#v, want zero layer", layer)
	}
}
