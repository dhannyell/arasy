package runtime

import (
	"math"

	"github.com/dhannyell/arasy/scene"
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

type seekKind uint8

const (
	seekBack seekKind = iota
	seekToTick
	seekBackSeconds
	seekScrub
)

type seekRequest struct {
	kind    seekKind
	tick    uint64
	seconds float64
}

type devControls struct {
	paused      bool
	speed       float64
	debugLayers scene.DebugLayers
	stepPending int
	seeks       []seekRequest
}

type devFrame struct {
	paused      bool
	speed       float64
	debugLayers scene.DebugLayers
	steps       int
	seeks       []seekRequest
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
		c.seeks = append(c.seeks, seekRequest{kind: seekBack})
	case ControlBackToTick:
		c.paused = true
		c.seeks = append(c.seeks, seekRequest{kind: seekToTick, tick: cmd.Tick})
	case ControlBackSeconds:
		c.paused = true
		c.seeks = append(c.seeks, seekRequest{kind: seekBackSeconds, seconds: cmd.Seconds})
	case ControlScrubToTick:
		c.paused = true
		c.seeks = append(c.seeks, seekRequest{kind: seekScrub, tick: cmd.Tick})
	case ControlReset:
		c.stepPending = 0
		c.seeks = c.seeks[:0]
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
	seeks := append([]seekRequest(nil), c.seeks...)
	c.seeks = c.seeks[:0]
	return devFrame{
		paused:      c.paused,
		speed:       speed,
		debugLayers: c.debugLayers,
		steps:       steps,
		seeks:       seeks,
	}
}
