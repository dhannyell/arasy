package runtime

import (
	"math"
	"time"
)

const defaultMaxCatchUp = 5

type clock struct {
	simDT       float64
	accumulator float64
	maxCatchUp  int
}

type stepPlan struct {
	Steps   int
	Clamped bool
}

// planSteps returns how many fixed simulation steps to run this render frame.
// It accumulates renderDT*speed and divides by simulationDT. At most max steps
// are returned; excess time is discarded when clamped. Zero max has no limit.
func planSteps(speed float64, renderDT, simulationDT float64, accumulator *float64, max int) stepPlan {
	if speed <= 0 || simulationDT <= 0 || renderDT <= 0 {
		return stepPlan{}
	}

	*accumulator += float64(renderDT * speed)

	steps := int(math.Floor(*accumulator / simulationDT))
	if max > 0 && steps > max {
		*accumulator = 0
		return stepPlan{
			Steps:   max,
			Clamped: true,
		}
	}

	*accumulator -= float64(float64(steps) * simulationDT)
	return stepPlan{Steps: steps}
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
func (c *clock) Consume(elapsed time.Duration, speed float64) stepPlan {
	return planSteps(speed, elapsed.Seconds(), c.simDT, &c.accumulator, c.maxCatchUp)
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
