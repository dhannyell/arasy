package wasm

func pendingInputMillis(current, candidate, now float64, eventCount int) float64 {
	if eventCount == 0 || !(candidate >= 0 && candidate <= now) {
		return current
	}
	if current < 0 || candidate < current {
		return candidate
	}
	return current
}
