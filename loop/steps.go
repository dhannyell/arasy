package loop

import "math"

type StepPlan struct {
	Steps   int
	Clamped bool
}

// Steps returns how many fixed simulation steps to run this render frame.
// It accumulates renderDT*speed and divides by simulationDT.
// At most max steps are returned; excess time is discarded when clamped.
// Pass max=0 to use no limit.
func Steps(speed float64, renderDT, simulationDT float64, accumulator *float64, max int) StepPlan {
	if speed <= 0 || simulationDT <= 0 || renderDT <= 0 {
		return StepPlan{}
	}

	*accumulator += renderDT * speed

	steps := int(math.Floor(*accumulator / simulationDT))
	if max > 0 && steps > max {
		*accumulator = 0
		return StepPlan{
			Steps:   max,
			Clamped: true,
		}
	}

	*accumulator -= float64(steps) * simulationDT

	return StepPlan{
		Steps: steps,
	}
}
