package input_test

import (
	"testing"

	"github.com/dhannyell/frameforge/input"
)

func BenchmarkMapperState(b *testing.B) {
	mapper := input.NewMapper([]input.Binding{
		{Source: keyboard(input.KeyD), Action: input.ActionMoveX, Scale: 1},
		{Source: keyboard(input.KeyA), Action: input.ActionMoveX, Scale: -1},
		{Source: keyboard(input.KeyW), Action: input.ActionMoveY, Scale: -1},
		{Source: keyboard(input.KeyS), Action: input.ActionMoveY, Scale: 1},
		{Source: keyboard(input.KeySpace), Action: input.ActionJump, Scale: 1},
	})

	mapper.Apply(input.Event{Source: keyboard(input.KeyD), Kind: input.EventButtonDown})
	mapper.Apply(input.Event{Source: keyboard(input.KeyW), Kind: input.EventButtonDown})

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		_ = mapper.State()
	}
}
