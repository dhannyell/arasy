package runtime

import (
	"math"

	"github.com/dhannyell/frameforge/scene"
)

const (
	minSpeed = 0.25
	maxSpeed = 8.0
)

type ControlKind uint8

const (
	ControlPlay ControlKind = iota
	ControlPause
	ControlStep
	ControlBack
	ControlBackToTick
	ControlBackSeconds
	ControlReset
	ControlSetSpeed
	ControlSetDebugLayers
	ControlScrubToTick
)

type ControlCommand struct {
	Kind        ControlKind
	Speed       float64
	Tick        uint64
	Seconds     float64
	DebugLayers scene.DebugLayers
}

type devControls struct {
	paused      bool
	speed       float64
	debugLayers scene.DebugLayers
	stepPending int
	backPending int
	backTicks   []uint64
	backSeconds []float64
	scrubTicks  []uint64
}

type devFrame struct {
	paused      bool
	speed       float64
	debugLayers scene.DebugLayers
	steps       int
	backs       int
	backTicks   []uint64
	backSeconds []float64
	scrubTicks  []uint64
}

func (c *devControls) apply(cmd ControlCommand) (isReset bool) {
	switch cmd.Kind {
	case ControlPlay:
		c.paused = false
	case ControlPause:
		c.paused = true
	case ControlStep:
		c.paused = true
		c.stepPending++
	case ControlBack:
		c.paused = true
		c.backPending++
	case ControlBackToTick:
		c.paused = true
		c.backTicks = append(c.backTicks, cmd.Tick)
	case ControlBackSeconds:
		c.paused = true
		c.backSeconds = append(c.backSeconds, cmd.Seconds)
	case ControlScrubToTick:
		c.paused = true
		c.scrubTicks = append(c.scrubTicks, cmd.Tick)
	case ControlReset:
		c.stepPending = 0
		c.backPending = 0
		c.backTicks = c.backTicks[:0]
		c.backSeconds = c.backSeconds[:0]
		c.scrubTicks = c.scrubTicks[:0]
		return true
	case ControlSetSpeed:
		if math.IsNaN(cmd.Speed) || cmd.Speed < minSpeed {
			cmd.Speed = minSpeed
		}
		if cmd.Speed > maxSpeed {
			cmd.Speed = maxSpeed
		}
		c.speed = cmd.Speed
	case ControlSetDebugLayers:
		c.debugLayers = cmd.DebugLayers
	}
	return false
}

// Consume reads and resets the volatile fields (stepPending, backPending).
// Called once per tick at the start of Advance.
func (c *devControls) consume() devFrame {
	speed := c.speed
	if c.paused {
		speed = 0
	}
	steps := c.stepPending
	c.stepPending = 0
	backs := c.backPending
	c.backPending = 0
	backTicks := append([]uint64(nil), c.backTicks...)
	c.backTicks = c.backTicks[:0]
	backSeconds := append([]float64(nil), c.backSeconds...)
	c.backSeconds = c.backSeconds[:0]
	// Coalesce scrub ticks: each ScrubToTick replays deterministically from
	// tick 0, so only the final target matters. When a platform queues many
	// scrub commands during a slider drag (the web canvas sends one every
	// 50 milliseconds), processing every intermediate target would replay from 0 that
	// many times -- keeping just the last tick collapses that to one replay.
	var scrubTicks []uint64
	if len(c.scrubTicks) > 0 {
		scrubTicks = []uint64{c.scrubTicks[len(c.scrubTicks)-1]}
	}
	c.scrubTicks = c.scrubTicks[:0]
	return devFrame{
		paused:      c.paused,
		speed:       speed,
		debugLayers: c.debugLayers,
		steps:       steps,
		backs:       backs,
		backTicks:   backTicks,
		backSeconds: backSeconds,
		scrubTicks:  scrubTicks,
	}
}
