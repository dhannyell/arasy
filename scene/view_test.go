package scene_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
	"github.com/dhannyell/frameforge/scene"
)

// forbiddenMutationNames are exact method names WorldView must never expose.
var forbiddenMutationNames = map[string]bool{
	"Spawn":   true,
	"Despawn": true,
	"Step":    true,
	"Clone":   true,
}

// TestWorldViewExposesNoMutators reflects over WorldView's method set and
// fails if any method name matches the mutation vocabulary — the whole
// point of the type is that this list stays empty.
func TestWorldViewExposesNoMutators(t *testing.T) {
	vt := reflect.TypeFor[scene.WorldView]()
	for i := 0; i < vt.NumMethod(); i++ {
		name := vt.Method(i).Name
		if strings.HasPrefix(name, "Set") || strings.HasPrefix(name, "Remove") || strings.HasPrefix(name, "Tween") || forbiddenMutationNames[name] {
			t.Errorf("WorldView exposes mutator-shaped method %q — Render must stay read-only", name)
		}
	}
	if vt.NumMethod() == 0 {
		t.Fatal("reflection found no WorldView methods — type name matching broke")
	}
}

// TestWorldViewRenderCommandsMatchesWorld guards that View() is a pure
// delegation: a view of a populated World returns the same commands as the
// World itself.
func TestWorldViewRenderCommandsMatchesWorld(t *testing.T) {
	world := scene.NewWorld()
	entity := world.Spawn()
	world.SetTransform(entity, scene.Transform{
		Position: math2d.Vec2{X: 4, Y: 7},
		Scale:    math2d.Vec2{X: 1, Y: 1},
	})
	world.SetCircleSprite(entity, scene.CircleSprite{
		Radius: 2,
		BaseSprite: scene.BaseSprite{
			Color: render.RGBA(1, 0, 0, 1),
			Layer: 5,
		},
	})

	want := world.RenderCommands()
	got := world.View().RenderCommands()

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("WorldView.RenderCommands() = %+v, want %+v", got, want)
	}
}
