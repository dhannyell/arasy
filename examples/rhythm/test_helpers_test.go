package rhythm

import "testing"

func testGameWithOnlyPending(ids ...int) *Game {
	game := &Game{}
	game.State.Health = 100
	for i := range game.State.Results {
		game.State.Results[i] = resultMiss
	}
	for _, id := range ids {
		game.State.Results[id] = resultPending
	}
	return game
}

func withTestChart(t *testing.T, fn func()) {
	t.Helper()
	original := chart
	t.Cleanup(func() { chart = original })
	fn()
}
