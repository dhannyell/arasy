package main

import (
	"github.com/dhannyell/frameforge/math2d"
)

func ballIntersectsGoal(center math2d.Vec2, radius float64, rect math2d.Rect) bool {
	closest := math2d.Vec2{
		X: clamp(center.X, rect.Min.X, rect.Max.X),
		Y: clamp(center.Y, rect.Min.Y, rect.Max.Y),
	}

	return center.DistanceSq(closest) <= radius*radius
}

func ballCrossedGoalLine(previous, current math2d.Vec2, radius float64, lineX float64, goalTop float64, goalBottom float64, leftSide bool) bool {
	crossX := lineX
	if leftSide {
		crossX += radius
	} else {
		crossX -= radius
	}

	deltaX := current.X - previous.X
	if deltaX == 0 {
		return false
	}

	t := (crossX - previous.X) / deltaX
	if t < 0 || t > 1 {
		return false
	}

	y := previous.Y + (current.Y-previous.Y)*t

	return y >= goalTop-radius && y <= goalBottom+radius
}

func clamp(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
