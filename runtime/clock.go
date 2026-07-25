package runtime

import (
	"math"
	"time"

	"github.com/dhannyell/arasy/loop"
)

const defaultMaxCatchUp = 5

type clock struct {
	simDT       float64
	accumulator float64
	maxCatchUp  int
}

func newClock(simulationHz int) *clock {
	if simulationHz <= 0 {
		simulationHz = defaultSimulationHz
	}

	return &clock{
		simDT:      1.0 / float64(simulationHz),
		maxCatchUp: defaultMaxCatchUp,
	}
}

// Consume converts elapsed wall time into fixed simulation steps.
func (c *clock) Consume(elapsed time.Duration, speed float64) loop.StepPlan {
	return loop.Steps(speed, elapsed.Seconds(), c.simDT, &c.accumulator, c.maxCatchUp)
}

func (c *clock) DT() float64 {
	return c.simDT
}

func (c *clock) Alpha() float64 {
	if c.simDT <= 0 {
		return 0
	}

	alpha := c.accumulator / c.simDT
	if alpha <= 0 {
		return 0
	}
	if alpha >= 1 {
		return math.Nextafter(1, 0)
	}
	return alpha
}

func (c *clock) Reset() {
	c.accumulator = 0
}
