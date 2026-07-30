// Package gpubuffer contains backend-independent policies for GPU buffers.
package gpubuffer

// InitialCapacity is the starting size used by streaming GPU buffers.
const InitialCapacity = 64 * 1024

// GrowCapacity returns the capacity required to hold required bytes.
//
// Growth uses power-of-two buckets and keeps one additional bucket of
// headroom. Animated streams commonly reveal adjacent shapes on consecutive
// frames; the headroom avoids replacing the active GPU buffer again on the
// next frame of a transition.
func GrowCapacity(current, required int) int {
	if current <= 0 {
		current = InitialCapacity
	}
	if required <= current {
		return current
	}
	for current < required {
		current *= 2
	}
	return current * 2
}
