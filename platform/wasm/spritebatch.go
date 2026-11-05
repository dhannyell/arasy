package wasm

import "github.com/dhannyell/arasy/internal/render/gpu/tess"

// spriteBatchAnimUniformSize is the shared tess uniform layout, so the wasm
// and native shaders read identical bytes.
const spriteBatchAnimUniformSize = tess.SpriteBatchAnimUniformSize

// uniformArena hands out aligned uniform slots for one frame's animated
// batch commands. Slots are never reused within a frame: queue.writeBuffer
// calls land before the command buffer, so two draws sharing one slot would
// both observe the last packed value.
type uniformArena struct {
	dataSize int
	slotSize int
	slots    int
	used     int
}

func newUniformArena(dataSize, alignment, slots int) uniformArena {
	slotSize := dataSize
	if rem := slotSize % alignment; rem != 0 {
		slotSize += alignment - rem
	}
	return uniformArena{dataSize: dataSize, slotSize: slotSize, slots: slots}
}

// alloc reserves the next slot and returns its byte offset. ok is false when
// the arena is full; the caller grows the backing buffer and rebuilds the
// bind groups that reference it.
func (a *uniformArena) alloc() (offset int, ok bool) {
	if a.used >= a.slots {
		return 0, false
	}
	offset = a.used * a.slotSize
	a.used++
	return offset, true
}

func (a *uniformArena) reset() {
	a.used = 0
}

// uniformArenaSlotCapacity returns the power-of-two slot capacity needed for
// required draws without shrinking current. The renderer calls this before it
// writes any per-frame uniforms so replacing the GPU buffer never invalidates
// slots already assigned to earlier draws in the same frame.
func uniformArenaSlotCapacity(current, required int) int {
	if current < 1 {
		current = 1
	}
	for current < required {
		current *= 2
	}
	return current
}
