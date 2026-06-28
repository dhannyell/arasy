package runtime

import "github.com/dhannyell/frameforge/scene"

type ControlKind uint8

const (
	ControlPlay ControlKind = iota
	ControlPause
	ControlStep
	ControlBack
	ControlReset
	ControlSetSpeed
	ControlSetDebugLayers
)

type ControlCommand struct {
	Kind        ControlKind
	Speed       int
	DebugLayers scene.DebugLayers
}

type devControls struct {
	paused      bool
	speed       int
	debugLayers scene.DebugLayers
	stepPending int
	backPending int
}

type devFrame struct {
	paused      bool
	speed       int
	debugLayers scene.DebugLayers
	reset       bool
	steps       int
	backs       int
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
	case ControlReset:
		c.stepPending = 0
		c.backPending = 0
		return true
	case ControlSetSpeed:
		if cmd.Speed < 1 {
			cmd.Speed = 1
		}
		if cmd.Speed > 8 {
			cmd.Speed = 8
		}
		c.speed = cmd.Speed
	case ControlSetDebugLayers:
		c.debugLayers = cmd.DebugLayers
	}
	return false
}

// consume reads and resets the volatile fields (stepPending, backPending).
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
	return devFrame{
		paused:      c.paused,
		speed:       speed,
		debugLayers: c.debugLayers,
		steps:       steps,
		backs:       backs,
	}
}
