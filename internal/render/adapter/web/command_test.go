package web

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
)

func TestNewCommandDTOsOrdersByLayer(t *testing.T) {
	commands := []render.Command{
		render.DrawPoint{BaseCommand: render.BaseCommand{LayerValue: 20}},
		render.DrawPoint{BaseCommand: render.BaseCommand{LayerValue: 5}},
		render.DrawPoint{BaseCommand: render.BaseCommand{LayerValue: 10}},
	}

	dtos := NewCommandDTOs(commands)

	got := []int{dtos[0].Layer, dtos[1].Layer, dtos[2].Layer}
	want := []int{5, 10, 20}

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("layers = %v, want %v", got, want)
		}
	}
}

func TestCommandDTOOmitsUnusedFields(t *testing.T) {
	dtos := NewCommandDTOs([]render.Command{
		render.DrawLine{
			From:  math2d.Vec2{X: 1, Y: 2},
			To:    math2d.Vec2{X: 3, Y: 4},
			Width: 0.5,
			Color: render.RGBA(1, 0, 0, 1),
		},
	})

	data, err := json.Marshal(dtos[0])
	if err != nil {
		t.Fatal(err)
	}

	jsonText := string(data)
	for _, field := range []string{`"rect"`, `"center"`, `"fill"`, `"stroke"`} {
		if strings.Contains(jsonText, field) {
			t.Fatalf("json %s should not contain unused field %s", jsonText, field)
		}
	}

	for _, field := range []string{`"from"`, `"to"`, `"color"`} {
		if !strings.Contains(jsonText, field) {
			t.Fatalf("json %s should contain field %s", jsonText, field)
		}
	}
}

func TestNewCommandDTOsSerializesDrawText(t *testing.T) {
	dtos := NewCommandDTOs([]render.Command{
		render.DrawText{
			Position:    math2d.Vec2{X: 12, Y: 34},
			Text:        "hello",
			Color:       render.RGBA(1, 1, 1, 1),
			Size:        2.5,
			BaseCommand: render.BaseCommand{LayerValue: 7},
		},
	})

	if len(dtos) != 1 {
		t.Fatalf("len(dtos) = %v, want 1", len(dtos))
	}

	dto := dtos[0]
	if dto.Type != "text" {
		t.Fatalf("Type = %q, want text", dto.Type)
	}

	if dto.Position == nil {
		t.Fatal("Position = nil, want value")
	}

	if dto.Position.X != 12 || dto.Position.Y != 34 {
		t.Fatalf("Position = %+v, want (12, 34)", dto.Position)
	}

	if dto.Text != "hello" {
		t.Fatalf("Text = %q, want hello", dto.Text)
	}

	if dto.Color == nil {
		t.Fatal("Color = nil, want value")
	}

	if dto.Size != 2.5 {
		t.Fatalf("Size = %v, want 2.5", dto.Size)
	}

	if dto.Layer != 7 {
		t.Fatalf("Layer = %v, want 7", dto.Layer)
	}
}

func TestNewCommandDTOsSerializesDrawImage(t *testing.T) {
	dtos := NewCommandDTOs([]render.Command{
		render.DrawImage{
			Texture:  1,
			Tint:     render.RGBA(1, 1, 1, 1),
			Dest:     math2d.NewRect(math2d.Vec2{X: 1, Y: 1}, math2d.Vec2{X: 2, Y: 2}),
			Src:      math2d.NewRect(math2d.Vec2{X: 1, Y: 1}, math2d.Vec2{X: 2, Y: 2}),
			Rotation: 40,
			BaseCommand: render.BaseCommand{
				LayerValue: 7,
				Clip:       math2d.NewRect(math2d.Vec2{X: 1, Y: 1}, math2d.Vec2{X: 2, Y: 2})},
		},
		render.DrawImage{
			Texture: 2,
			Dest:    math2d.NewRect(math2d.Vec2{X: 1, Y: 1}, math2d.Vec2{X: 2, Y: 2}),
		},
	})

	if len(dtos) != 2 {
		t.Fatalf("len(dtos) = %v, want 2", len(dtos))
	}

	if dtos[0].Texture != 2 {
		t.Fatalf("Texture = %v, want 2", dtos[0].Texture)
	}

	if dtos[0].Tint != nil {
		t.Fatalf("Tint = %v, want nil", dtos[0].Tint)
	}

	if dtos[0].Src != nil {
		t.Fatalf("Src = %v, want nil", dtos[0].Src)
	}

	if dtos[1].Texture != 1 {
		t.Fatalf("Texture = %v, want 1", dtos[1].Texture)
	}

	wantTint := ColorDTO{1, 1, 1, 1}
	if *dtos[1].Tint != wantTint {
		t.Fatalf("Tint = %v, want = %v", dtos[1].Tint, &wantTint)
	}
}
