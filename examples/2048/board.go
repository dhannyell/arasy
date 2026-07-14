// Package twenty48 implements the 2048 demo game.
package twenty48

const (
	gridSize       = 4
	boardCellCount = gridSize * gridSize
)

type board [boardCellCount]uint64

type direction int

const (
	dirLeft direction = iota
	dirRight
	dirUp
	dirDown
)

type tileMotion struct {
	From  int
	To    int
	Value uint64
}

type moveTrace struct {
	Motions [boardCellCount]tileMotion
	Count   int
	Merges  [boardCellCount]bool
}

func (b board) moveWithTrace(dir direction) (board, uint64, bool, moveTrace) {
	next := b
	var gained uint64
	moved := false
	trace := moveTrace{}

	for n := 0; n < gridSize; n++ {
		line := b.readLine(dir, n)
		var indices [gridSize]int
		for i := range indices {
			indices[i] = lineIndex(dir, n, i)
		}
		result, lineScore, lineMoved, lineTrace := slideMergeLeftWithTrace(line, indices)

		next.writeLine(dir, n, result)
		gained += lineScore
		moved = moved || lineMoved
		for i := 0; i < lineTrace.Count; i++ {
			trace.Motions[trace.Count] = lineTrace.Motions[i]
			trace.Count++
		}
		for i, merged := range lineTrace.Merges {
			trace.Merges[i] = trace.Merges[i] || merged
		}
	}

	return next, gained, moved, trace
}

func (b board) emptyCells() []int {
	empty := make([]int, 0, len(b))

	for i, value := range b {
		if value == 0 {
			empty = append(empty, i)
		}
	}

	return empty
}

func (b board) canMove() bool {
	for row := 0; row < gridSize; row++ {
		for col := 0; col < gridSize; col++ {
			index := row*gridSize + col
			value := b[index]

			if value == 0 {
				return true
			}

			if col < gridSize-1 && value == b[index+1] {
				return true
			}

			if row < gridSize-1 && value == b[index+gridSize] {
				return true
			}
		}
	}

	return false
}

func (b board) hasValue(want uint64) bool {
	for _, value := range b {
		if value == want {
			return true
		}
	}

	return false
}

func (b board) readLine(dir direction, n int) [gridSize]uint64 {
	var line [gridSize]uint64

	for i := 0; i < gridSize; i++ {
		line[i] = b[lineIndex(dir, n, i)]
	}

	return line
}

func (b *board) writeLine(dir direction, n int, line [gridSize]uint64) {
	for i := 0; i < gridSize; i++ {
		b[lineIndex(dir, n, i)] = line[i]
	}
}

func lineIndex(dir direction, n, offset int) int {
	switch dir {
	case dirLeft:
		return n*gridSize + offset
	case dirRight:
		return n*gridSize + (gridSize - 1 - offset)
	case dirUp:
		return offset*gridSize + n
	case dirDown:
		return (gridSize-1-offset)*gridSize + n
	default:
		panic("invalid 2048 direction")
	}
}

// slideMergeLeftWithTrace compacts a line toward its leading edge and merges
// equal adjacent tiles. Each tile can participate in at most one merge.
func slideMergeLeftWithTrace(line [gridSize]uint64, indices [gridSize]int) ([gridSize]uint64, uint64, bool, moveTrace) {
	var compact [gridSize]tileMotion
	count := 0
	for i, value := range line {
		if value == 0 {
			continue
		}
		compact[count] = tileMotion{From: indices[i], Value: value}
		count++
	}

	var result [gridSize]uint64
	trace := moveTrace{}
	var gained uint64
	out := 0
	for i := 0; i < count; {
		first := compact[i]
		destination := indices[out]
		if i+1 < count && first.Value == compact[i+1].Value {
			second := compact[i+1]
			result[out] = first.Value * 2
			gained += result[out]
			trace.Motions[trace.Count] = tileMotion{From: first.From, To: destination, Value: first.Value}
			trace.Count++
			trace.Motions[trace.Count] = tileMotion{From: second.From, To: destination, Value: second.Value}
			trace.Count++
			trace.Merges[destination] = true
			i += 2
		} else {
			result[out] = first.Value
			if first.From != destination {
				trace.Motions[trace.Count] = tileMotion{From: first.From, To: destination, Value: first.Value}
				trace.Count++
			}
			i++
		}
		out++
	}

	return result, gained, result != line, trace
}
