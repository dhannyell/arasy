package vector

import (
	"reflect"
	"testing"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
)

func TestAppendFillWithWorkspaceMatchesStatelessOutputAndReusesBuffers(t *testing.T) {
	var pathBuilder Builder
	pathBuilder.MoveTo(math2d.Vec2{})
	pathBuilder.LineTo(math2d.Vec2{X: 20})
	pathBuilder.LineTo(math2d.Vec2{X: 25, Y: 10})
	pathBuilder.LineTo(math2d.Vec2{X: 10, Y: 20})
	pathBuilder.Close()
	path, err := pathBuilder.Build()
	if err != nil {
		t.Fatal(err)
	}

	style := FillStyle{Color: render.RGBA8(40, 120, 220, 255), Tolerance: 0.25}
	want, err := path.AppendFill(nil, render.BaseCommand{}, style)
	if err != nil {
		t.Fatal(err)
	}
	var workspace FillWorkspace
	got, err := path.AppendFillWithWorkspace(nil, render.BaseCommand{}, style, &workspace)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatal("workspace output differs from stateless output")
	}

	firstPoint := &workspace.points[:cap(workspace.points)][0]
	firstIndex := &workspace.indices[:cap(workspace.indices)][0]
	firstOrder := &workspace.order[:cap(workspace.order)][0]
	if _, err := path.AppendFillWithWorkspace(nil, render.BaseCommand{}, style, &workspace); err != nil {
		t.Fatal(err)
	}
	if &workspace.points[:cap(workspace.points)][0] != firstPoint {
		t.Fatal("point buffer was reallocated")
	}
	if &workspace.indices[:cap(workspace.indices)][0] != firstIndex {
		t.Fatal("index buffer was reallocated")
	}
	if &workspace.order[:cap(workspace.order)][0] != firstOrder {
		t.Fatal("triangulation buffer was reallocated")
	}
}
