package scene_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
	"github.com/dhannyell/arasy/scene"
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

// TestWorldViewMirrorsWorldComponentReaders is the other half of the guard
// above. TestWorldViewExposesNoMutators checks that nothing extra leaks into
// the view; this checks that nothing is missing from it. Without it the view
// silently fell five components behind World when the 2.5D work landed, and a
// game's Render simply could not read an entity's height, spin, ballistics,
// locomotion or move intent.
func TestWorldViewMirrorsWorldComponentReaders(t *testing.T) {
	worldType := reflect.TypeFor[*scene.World]()
	viewType := reflect.TypeFor[scene.WorldView]()
	entityType := reflect.TypeFor[scene.EntityID]()
	boolType := reflect.TypeFor[bool]()

	readers := 0
	for i := range worldType.NumMethod() {
		method := worldType.Method(i)
		// Component reader shape: func(EntityID) (T, bool). Index 0 of a
		// method obtained from a type is the receiver.
		signature := method.Type
		if signature.NumIn() != 2 || signature.In(1) != entityType {
			continue
		}
		if signature.NumOut() != 2 || signature.Out(1) != boolType {
			continue
		}
		readers++

		viewMethod, ok := viewType.MethodByName(method.Name)
		if !ok {
			t.Errorf("World exposes component reader %s but WorldView does not: "+
				"a game's Render cannot read that component", method.Name)
			continue
		}
		if got, want := viewMethod.Type.Out(0), signature.Out(0); got != want {
			t.Errorf("WorldView.%s returns %s, World returns %s", method.Name, got, want)
		}
	}

	if readers == 0 {
		t.Fatal("reflection found no component readers on World — shape matching broke")
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
