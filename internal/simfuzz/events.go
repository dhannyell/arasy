package simfuzz

import (
	"encoding/binary"
	"math"

	"github.com/dhannyell/arasy/input"
)

// fuzzBoundSources are the keyboard sources the fuzz game binds. Every decoded
// event targets one of these so input actually reaches the simulation instead
// of being dropped by the mapper as an unbound key.
var fuzzBoundSources = []input.Source{
	input.Keyboard(input.KeyArrowLeft),
	input.Keyboard(input.KeyArrowRight),
	input.Keyboard(input.KeyArrowUp),
	input.Keyboard(input.KeyArrowDown),
	input.Keyboard(input.KeySpace),
}

// eventRecordLen is the per-event byte width decoded from the script tail.
const eventRecordLen = 11

// decodeEvents reads a monotonic input event log from data starting at offset.
// Each event targets a bound key (so it drives the sim) with a kind and a tick
// delta; ticks accumulate and are clamped to [0, maxTicks-1]. Truncated records
// are zero-padded. The result is total: never panics, always valid events.
func decodeEvents(data []byte, offset, maxTicks int) []input.Event {
	if offset >= len(data) || maxTicks <= 0 {
		return nil
	}
	count := int(readU8At(data, offset))
	offset++
	if count > maxFuzzEvents {
		count = maxFuzzEvents
	}
	events := make([]input.Event, 0, count)
	tick := 0
	for i := 0; i < count; i++ {
		rec := readBytes(data, &offset, eventRecordLen)
		keyIdx := int(rec[0]) % len(fuzzBoundSources)
		kind := input.EventKind(rec[1] % 2) // Down / Up
		delta := int(rec[2])
		value := math.Float64frombits(binary.LittleEndian.Uint64(rec[3:11]))
		tick += delta
		if tick >= maxTicks {
			tick = maxTicks - 1
		}
		if tick < 0 {
			tick = 0
		}
		events = append(events, input.Event{
			Tick:   uint64(tick),
			Source: fuzzBoundSources[keyIdx],
			Kind:   kind,
			Value:  value,
		})
	}
	return events
}

// groupEventsByTick buckets events by their Tick field.
func groupEventsByTick(events []input.Event) map[uint64][]input.Event {
	byTick := make(map[uint64][]input.Event)
	for _, e := range events {
		byTick[e.Tick] = append(byTick[e.Tick], e)
	}
	return byTick
}

// arbitraryEventRecordLen is the per-event width for the P6 arbitrary stream.
const arbitraryEventRecordLen = 29

// decodeArbitraryEvents builds a stream of possibly-invalid input events
// directly from raw bytes (any device, any code, any kind, arbitrary value and
// pointer coordinates). P6 feeds these to the engine to prove arbitrary input
// never panics. Invalid events are silently dropped by the mapper; the property
// under test is "no panic", asserted by the fuzz framework recovering panics.
func decodeArbitraryEvents(data []byte, maxTicks int) []input.Event {
	if maxTicks <= 0 {
		return nil
	}
	var events []input.Event
	for i := 0; i+arbitraryEventRecordLen <= len(data) && len(events) < maxFuzzEvents; i += arbitraryEventRecordLen {
		rec := data[i : i+arbitraryEventRecordLen]
		tick := uint64(rec[0]) % uint64(maxTicks)
		events = append(events, input.Event{
			Tick: tick,
			Source: input.Source{
				Device: input.DeviceKind(rec[1] % 4),
				Code:   input.SourceCode(binary.LittleEndian.Uint16(rec[2:4])),
			},
			Kind:  input.EventKind(rec[4] % 5),
			Value: math.Float64frombits(binary.LittleEndian.Uint64(rec[5:13])),
			X:     math.Float64frombits(binary.LittleEndian.Uint64(rec[13:21])),
			Y:     math.Float64frombits(binary.LittleEndian.Uint64(rec[21:29])),
		})
	}
	return events
}

// readU8At reads a single byte at a fixed offset without advancing a cursor.
func readU8At(data []byte, offset int) uint8 {
	if offset >= len(data) {
		return 0
	}
	return data[offset]
}

// maxScrubHops caps the number of scrub jumps decoded for P8 so a case cannot
// spend the whole fuzz budget seeking.
const maxScrubHops = 16

// decodeScrubTargets derives a sequence of scrub target ticks from the raw
// bytes, each in [0, maxTick]. It reads from the tail of the input (the part
// after the entity/event section) so it does not collide with the script
// decoder, and always ends with maxTick so the final hop returns to the live
// head. Targets are total: any bytes produce valid, in-range ticks.
func decodeScrubTargets(data []byte, maxTick int) []uint64 {
	if maxTick <= 0 {
		return nil
	}
	n := max(min(maxScrubHops, len(data)), 2)
	// Sample bytes from the end of the input; skip the last byte so this does
	// not race with decodeArbitraryEvents on the same region in P6 (different
	// targets, but keep the regions disjoint for clarity).
	start := max(len(data)-n-1, 0)
	targets := make([]uint64, 0, n)
	for i := 0; i < n && start+i < len(data); i++ {
		b := data[start+i]
		targets = append(targets, uint64(int(b)%maxTick))
	}
	targets = append(targets, uint64(maxTick))
	return targets
}
