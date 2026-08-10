package tilemap

import "fmt"

// ParseASCII converts a row-oriented text map into a tile layer.
// Each rune must have an entry in legend, and all rows must have the same
// number of runes.
func ParseASCII(rows []string, legend map[rune]int16) (Layer, error) {
	layer := Layer{H: len(rows)}
	if len(rows) == 0 {
		return layer, nil
	}

	width := len([]rune(rows[0]))
	layer.W = width
	layer.Tiles = make([]int16, 0, width*len(rows))
	for y, row := range rows {
		cells := []rune(row)
		if len(cells) != width {
			return Layer{}, fmt.Errorf("tilemap: row %d has width %d, want %d", y, len(cells), width)
		}
		for x, cell := range cells {
			tile, ok := legend[cell]
			if !ok {
				return Layer{}, fmt.Errorf("tilemap: unknown rune %q at row %d, column %d", cell, y, x)
			}
			layer.Tiles = append(layer.Tiles, tile)
		}
	}
	return layer, nil
}
