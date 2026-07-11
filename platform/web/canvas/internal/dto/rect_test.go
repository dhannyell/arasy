package dto

import (
	"testing"

	"github.com/dhannyell/frameforge/math2d"
)

func TestNewRectDTOUsesPositionAndSize(t *testing.T) {
	rect := math2d.NewRect(
		math2d.Vec2{X: 20, Y: 30},
		math2d.Vec2{X: 40, Y: 50},
	)

	dto := NewRectDTO(rect)

	if dto.X != 20 {
		t.Fatalf("X = %v, want 20", dto.X)
	}

	if dto.Y != 30 {
		t.Fatalf("Y = %v, want 30", dto.Y)
	}

	if dto.Width != 40 {
		t.Fatalf("Width = %v, want 40", dto.Width)
	}

	if dto.Height != 50 {
		t.Fatalf("Height = %v, want 50", dto.Height)
	}
}
