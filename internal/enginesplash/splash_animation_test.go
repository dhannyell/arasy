package enginesplash

import (
	"testing"

	"github.com/dhannyell/arasy/anim"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
)

func TestArasySplashDawnOpacityIsContinuous(t *testing.T) {
	viewport := math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 1920, Y: 1080})
	var previous float64
	for frame := 50; frame <= 72; frame++ {
		commands := AppendArasySplash(nil, anim.At(float64(frame)/50), viewport, ArasySplashResources{})
		var sunAlpha float64
		for _, command := range commands {
			circle, ok := command.(render.DrawCircle)
			if ok && circle.Radius == 136 && circle.Fill.R > 0.94 && circle.Fill.G > 0.7 {
				sunAlpha = sunAlpha + circle.Fill.A*(1-sunAlpha)
			}
		}
		if frame > 50 && sunAlpha+1e-9 < previous {
			t.Fatalf("sun opacity fell at frame %d: %v -> %v", frame, previous, sunAlpha)
		}
		previous = sunAlpha
	}
}
