package enginesplash

import (
	"reflect"
	"testing"

	"github.com/dhannyell/arasy/anim"
	"github.com/dhannyell/arasy/math2d"
)

func TestArasySplashWorkspaceMatchesStatelessDuringOrbit(t *testing.T) {
	viewport := math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 1920, Y: 1080})
	var workspace ArasySplashWorkspace
	for frame := 0; frame <= 50; frame++ {
		timeline := anim.At(float64(frame) / 50)
		want := AppendArasySplash(nil, timeline, viewport, ArasySplashResources{})
		got := workspace.Append(timeline, viewport, ArasySplashResources{})
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("frame %d workspace output differs from stateless output", frame)
		}
	}
}
