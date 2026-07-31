//go:build arr_dev

package devpanel

import (
	"testing"
	"time"

	"github.com/dhannyell/arasy/render"
)

func TestArmedPickOutlinesPickableEntities(t *testing.T) {
	p := newInspectPanel(t)
	armPick(t, p)

	cmds := p.Render(frameCamera(), 16*time.Millisecond, Stats{})
	circles := 0
	for _, cmd := range cmds {
		circle, ok := cmd.(render.DrawCircle)
		if !ok {
			continue
		}
		circles++
		if circle.Layer() < layerBase {
			t.Errorf("pick outline at layer %d, below the overlay band", circle.Layer())
		}
		if circle.Stroke != colorAction {
			t.Errorf("pick outline stroke = %v, want the action color", circle.Stroke)
		}
		// The ball sits at ~(3,4) in an identity camera; radius 5.
		if circle.Center.X < 2 || circle.Center.X > 5 || circle.Center.Y < 3 || circle.Center.Y > 5 {
			t.Errorf("pick outline at %v, want it over the ball at ~(3,4)", circle.Center)
		}
		if circle.Radius != 5 {
			t.Errorf("outline radius = %v, want the collider radius 5", circle.Radius)
		}
	}
	// Only the ball has a collider; the keeper must not glow.
	if circles != 1 {
		t.Fatalf("drew %d pick outlines, want exactly the ball's", circles)
	}
}

func TestDisarmedPickDrawsNoOutlines(t *testing.T) {
	p := newInspectPanel(t)
	cmds := renderTab(p)
	for _, cmd := range cmds {
		if circle, ok := cmd.(render.DrawCircle); ok && circle.Stroke == colorAction {
			t.Fatal("drew a pick outline with the gesture disarmed")
		}
	}
}
