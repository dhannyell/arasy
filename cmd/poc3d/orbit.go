package main

import (
	"math"

	"github.com/dhannyell/arasy/dmath"
	"github.com/dhannyell/arasy/input"
	"github.com/dhannyell/arasy/internal/math3d"
	"github.com/dhannyell/arasy/math2d"
)

const (
	orbitYawSpeed   = 0.005 // radians per dragged pixel
	orbitPitchSpeed = 0.005
	orbitZoomSpeed  = 0.1 // distance fraction per wheel notch

	// pitchLimit keeps the camera away from the poles, where the up
	// vector flips.
	pitchLimit = 85 * math.Pi / 180

	minDistance = 30
	maxDistance = 300

	// orbitSmoothing is the fraction of the remaining gap closed per tick.
	orbitSmoothing = 0.25
)

// orbitCamera orbits a target from yaw/pitch/distance. Presentation data only.
type orbitCamera struct {
	Target   math3d.Vec3
	Yaw      float64 // radians around Z
	Pitch    float64 // radians above the ground plane
	Distance float64

	// Smoothed copies trail the fields above and drive the projection.
	smoothYaw      float64
	smoothPitch    float64
	smoothDistance float64

	lastPointer math2d.Vec2
	dragging    bool
}

// newOrbitCamera starts at the given pose with smoothing already settled.
func newOrbitCamera(target math3d.Vec3, yaw, pitch, distance float64) orbitCamera {
	return orbitCamera{
		Target:         target,
		Yaw:            yaw,
		Pitch:          pitch,
		Distance:       distance,
		smoothYaw:      yaw,
		smoothPitch:    pitch,
		smoothDistance: distance,
	}
}

func (o *orbitCamera) update(frame input.PointerFrame) {
	if frame.Buttons[input.PointerPrimary] && frame.Inside {
		if o.dragging {
			delta := frame.Pos.Sub(o.lastPointer)
			o.Yaw -= delta.X * orbitYawSpeed
			// Screen Y grows down; dragging up must raise the camera.
			o.Pitch -= delta.Y * orbitPitchSpeed
			o.Pitch = math.Min(math.Max(o.Pitch, -pitchLimit), pitchLimit)
		}
		o.dragging = true
		o.lastPointer = frame.Pos
	} else {
		o.dragging = false
	}

	if frame.Wheel.Y != 0 {
		o.Distance *= 1 - frame.Wheel.Y*orbitZoomSpeed
		o.Distance = math.Min(math.Max(o.Distance, minDistance), maxDistance)
	}

	o.smoothYaw += (o.Yaw - o.smoothYaw) * orbitSmoothing
	o.smoothPitch += (o.Pitch - o.smoothPitch) * orbitSmoothing
	o.smoothDistance += (o.Distance - o.smoothDistance) * orbitSmoothing
}

func (o *orbitCamera) camera(viewport math2d.Rect) math3d.Camera {
	dir := math3d.Vec3{
		X: dmath.Cos(o.smoothPitch) * dmath.Cos(o.smoothYaw),
		Y: dmath.Cos(o.smoothPitch) * dmath.Sin(o.smoothYaw),
		Z: dmath.Sin(o.smoothPitch),
	}
	return math3d.Camera{
		Eye:      o.Target.Add(dir.Mul(o.smoothDistance)),
		Target:   o.Target,
		FOV:      math.Pi / 3,
		Near:     0.1,
		Viewport: viewport,
	}
}
