package simfuzz

import (
	"github.com/dhannyell/arasy/input"
	"github.com/dhannyell/arasy/math2d"
)

// MaxEntities is the maximum number of entities a script may describe.
const MaxEntities = 256

// MaxTicks is the maximum number of simulation ticks a script may request.
const MaxTicks = 600

// maxFuzzEvents caps the number of input events decoded from a script.
const maxFuzzEvents = 128

// fuzzBounds is the fixed simulation bounds used by the fuzz game.
var fuzzBounds = math2d.NewRect(math2d.Vec2{X: -200, Y: -200}, math2d.Vec2{X: 200, Y: 200})

// Script describes a deterministic simulation scenario: a seed, entities with
// components, a tick count, and a monotonic input event log. DecodeScript is a
// total function over arbitrary byte slices.
type Script struct {
	Seed     uint64
	Version  uint8
	Flags    uint8
	Entities []EntityDef
	MaxTicks int
	Events   []input.Event
}

// EntityDef describes one entity and its packed component data.
type EntityDef struct {
	ComponentMask uint32
	Data          []byte
}

// Component bits in EntityDef.ComponentMask, in the order SpawnFromScript reads
// them. compDataLen must stay in sync with this order, and a structural guard
// test (TestSpawnCoversAllDecodedComponents) fails if a declared bit is never
// spawned.
const (
	CompTransform      uint32 = 1 << 0
	CompVelocity       uint32 = 1 << 1
	CompAcceleration   uint32 = 1 << 2
	CompDamping        uint32 = 1 << 3
	CompBody           uint32 = 1 << 4
	CompCircleCollider uint32 = 1 << 5
	CompRectCollider   uint32 = 1 << 6
	CompCircleSprite   uint32 = 1 << 7
	CompHeight         uint32 = 1 << 8
	CompSpin           uint32 = 1 << 9
	CompBallistics     uint32 = 1 << 10
	CompLocomotion     uint32 = 1 << 11
	CompMoveIntent     uint32 = 1 << 12
	CompImageSprite    uint32 = 1 << 13
	CompTransformTween uint32 = 1 << 14
	CompCircleTween    uint32 = 1 << 15
)

// allComponents is the union of every known component bit. Unknown bits are
// dropped so future components do not silently consume data.
const allComponents = CompTransform | CompVelocity | CompAcceleration |
	CompDamping | CompBody | CompCircleCollider | CompRectCollider |
	CompCircleSprite | CompHeight | CompSpin | CompBallistics |
	CompLocomotion | CompMoveIntent | CompImageSprite |
	CompTransformTween | CompCircleTween

// DecodeScript decodes a Script from fuzzer bytes. It is total and
// deterministic: no error, no panic, any byte sequence is valid, and the same
// bytes always produce the same Script. Missing bytes default to zero.
//
// Version 0 layout: [0]version [1]flags [2]entityCount [3]maxTicks.
// Version 1 widens maxTicks to uint16 at [3:5], making the documented
// 600-tick ceiling reachable. Both layouts then store per-entity masks/data,
// followed by the input event log.
func DecodeScript(data []byte) Script {
	s := Script{MaxTicks: 10}
	if len(data) == 0 {
		s.Seed = seedFromBytes(data)
		return s
	}
	s.Version = data[0]
	if len(data) >= 2 {
		s.Flags = data[1]
	}
	maxEnt := 0
	if len(data) >= 3 {
		maxEnt = int(data[2])
	}
	offset := 4
	if s.Version >= 1 {
		offset = 5
		var ticks uint16
		if len(data) >= 4 {
			ticks = uint16(data[3])
		}
		if len(data) >= 5 {
			ticks |= uint16(data[4]) << 8
		}
		s.MaxTicks = clamp(int(ticks), 1, MaxTicks)
	} else if len(data) >= 4 {
		s.MaxTicks = clamp(int(data[3]), 1, MaxTicks)
	}
	s.Seed = seedFromBytes(data)

	n := min(maxEnt, MaxEntities)
	for i := 0; i < n && offset < len(data); i++ {
		mask := readMask(data, &offset) & allComponents
		dataLen := compDataLen(mask)
		compData := readBytes(data, &offset, dataLen)
		s.Entities = append(s.Entities, EntityDef{ComponentMask: mask, Data: compData})
	}

	s.Events = decodeEvents(data, offset, s.MaxTicks)
	return s
}

// compDataLen returns the number of component data bytes needed for the given
// mask. The field order matches SpawnFromScript exactly; the guard test
// TestCompDataLenMatchesSpawn ensures the two never drift.
func compDataLen(mask uint32) int {
	var n int
	if mask&CompTransform != 0 {
		n += 40 // 5*float64
	}
	if mask&CompVelocity != 0 {
		n += 16 // 2*float64
	}
	if mask&CompAcceleration != 0 {
		n += 16 // 2*float64
	}
	if mask&CompDamping != 0 {
		n += 8 // 1*float64
	}
	if mask&CompBody != 0 {
		n += 16 // 2*float64
	}
	if mask&CompCircleCollider != 0 {
		n += 8 // 1*float64
	}
	if mask&CompRectCollider != 0 {
		n += 16 // 2*float64
	}
	if mask&CompCircleSprite != 0 {
		n += 41 // 1*uint8 + 5*float64
	}
	if mask&CompHeight != 0 {
		n += 16 // 2*float64
	}
	if mask&CompSpin != 0 {
		n += 8 // 1*float64
	}
	if mask&CompBallistics != 0 {
		n += 68 // 8*float64 + 1*int32
	}
	if mask&CompLocomotion != 0 {
		n += 32 // 4*float64
	}
	if mask&CompMoveIntent != 0 {
		n += 24 // 3*float64
	}
	if mask&CompImageSprite != 0 {
		n += 88 // 2*uint32 + 10*float64
	}
	if mask&CompTransformTween != 0 {
		n += 100 // 2*(5*float64) + 2*float64 + 4*uint8
	}
	if mask&CompCircleTween != 0 {
		n += 102 // 2*(1*uint8+5*float64) + 2*float64 + 4*uint8
	}
	return n
}
