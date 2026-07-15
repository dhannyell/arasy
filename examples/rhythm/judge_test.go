package rhythm

import (
	"testing"

	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/runtime"
)

func TestJudgeDeltaWindows(t *testing.T) {
	cases := []struct {
		delta int
		want  objectResult
	}{
		{delta: 0, want: resultMarvelous},
		{delta: 22, want: resultMarvelous},
		{delta: 23, want: resultPerfect},
		{delta: -45, want: resultPerfect},
		{delta: 46, want: resultGood},
		{delta: 90, want: resultGood},
		{delta: 91, want: resultBad},
		{delta: 135, want: resultBad},
		{delta: 136, want: resultMiss},
	}
	for _, tc := range cases {
		if got := judgeDelta(tc.delta); got != tc.want {
			t.Errorf("judgeDelta(%d) = %d, want %d", tc.delta, got, tc.want)
		}
	}
}

func TestTapChoosesClosestPendingNote(t *testing.T) {
	withTestChart(t, func() {
		chart[0] = chartObject{ID: 0, Kind: objectTap, TimeMS: 940, Lane: 1}
		chart[1] = chartObject{ID: 1, Kind: objectTap, TimeMS: 1010, Lane: 1}
		game := testGameWithOnlyPending(0, 1)

		game.tryHitLane(&runtime.UpdateContext{}, 1, 1000)

		if got := game.State.Results[1]; got != resultMarvelous {
			t.Fatalf("closest note result = %d, want marvelous", got)
		}
		if got := game.State.Results[0]; got != resultPending {
			t.Fatalf("farther note result = %d, want pending", got)
		}
	})
}

func TestHoldFailsWhenReleasedEarly(t *testing.T) {
	withTestChart(t, func() {
		chart[0] = chartObject{ID: 0, Kind: objectHold, TimeMS: 1000, EndMS: 1800, Lane: 0}
		game := testGameWithOnlyPending(0)
		ctx := &runtime.UpdateContext{}

		game.tryHitLane(ctx, 0, 1000)
		if got := game.State.Results[0]; got != resultHeld {
			t.Fatalf("hold start = %d, want held", got)
		}
		game.releaseLane(ctx, 0, 1600)
		if got := game.State.Results[0]; got != resultMiss {
			t.Fatalf("early release = %d, want miss", got)
		}
	})
}

func TestHoldCompletesAtEnd(t *testing.T) {
	withTestChart(t, func() {
		chart[0] = chartObject{ID: 0, Kind: objectHold, TimeMS: 1000, EndMS: 1800, Lane: 0}
		game := testGameWithOnlyPending(0)
		ctx := &runtime.UpdateContext{}

		game.tryHitLane(ctx, 0, 1000)
		game.finishHolds(ctx, 1800)
		if got := game.State.Results[0]; got != resultPerfect {
			t.Fatalf("completed hold = %d, want perfect", got)
		}
	})
}

func TestCircleRequiresPointerInsideRadius(t *testing.T) {
	withTestChart(t, func() {
		chart[0] = chartObject{ID: 0, Kind: objectCircle, TimeMS: 1000, Lane: -1, Pos: math2d.Vec2{X: 40, Y: 30}, Radius: 3}
		game := testGameWithOnlyPending(0)
		ctx := &runtime.UpdateContext{}

		game.tryHitCircle(ctx, math2d.Vec2{X: 44, Y: 30}, 1000)
		if got := game.State.Results[0]; got != resultPending {
			t.Fatalf("outside click = %d, want pending", got)
		}
		game.tryHitCircle(ctx, math2d.Vec2{X: 42, Y: 30}, 1000)
		if got := game.State.Results[0]; got != resultMarvelous {
			t.Fatalf("inside click = %d, want marvelous", got)
		}
	})
}

func TestLanePromptLevelTracksTheHitWindows(t *testing.T) {
	withTestChart(t, func() {
		chart[0] = chartObject{ID: 0, Kind: objectTap, TimeMS: 1000, Lane: 2}
		game := testGameWithOnlyPending(0)

		if got := lanePromptLevel(game.State, 910, 2); got != 1 {
			t.Fatalf("good-window prompt = %d, want 1", got)
		}
		if got := lanePromptLevel(game.State, 1000, 2); got != 2 {
			t.Fatalf("perfect-window prompt = %d, want 2", got)
		}
		if got := lanePromptLevel(game.State, 1091, 2); got != 0 {
			t.Fatalf("outside prompt window = %d, want 0", got)
		}
	})
}

func TestLaneHasActiveHold(t *testing.T) {
	withTestChart(t, func() {
		chart[0] = chartObject{ID: 0, Kind: objectHold, TimeMS: 1000, EndMS: 1600, Lane: 2}
		game := testGameWithOnlyPending(0)
		game.State.Results[0] = resultHeld

		if !laneHasActiveHold(game.State, 2) {
			t.Fatal("held note did not activate its lane")
		}
		if laneHasActiveHold(game.State, 1) {
			t.Fatal("hold activated the wrong lane")
		}
	})
}

func TestChartNoteSuppressedDuringEarlierHoldOnSameLane(t *testing.T) {
	withTestChart(t, func() {
		chart[0] = chartObject{ID: 0, Kind: objectHold, TimeMS: 1000, EndMS: 1800, Lane: 1}
		chart[1] = chartObject{ID: 1, Kind: objectTap, TimeMS: 1200, Lane: 1}
		chart[2] = chartObject{ID: 2, Kind: objectTap, TimeMS: 1200, Lane: 2}

		if !chartNoteSuppressed(1) {
			t.Fatal("tap on an active hold lane was not suppressed")
		}
		if chartNoteSuppressed(2) {
			t.Fatal("tap on a different lane was suppressed")
		}
	})
}

func TestSpecialPhraseDoesNotCauseMiss(t *testing.T) {
	withTestChart(t, func() {
		chart[0] = chartObject{ID: 0, Kind: objectSpecial, TimeMS: 1000, EndMS: 1600, Lane: -1}
		game := testGameWithOnlyPending(0)

		game.updateMisses(&runtime.UpdateContext{}, 2000)
		if got := game.State.Results[0]; got != resultPending {
			t.Fatalf("special phrase result = %d, want pending metadata", got)
		}
	})
}

func TestLightningOnlyStartsComboOrHitsSpecial(t *testing.T) {
	withTestChart(t, func() {
		ctx := &runtime.UpdateContext{}

		chart[0] = chartObject{ID: 0, Kind: objectTap, TimeMS: 1000, Lane: 1}
		normal := testGameWithOnlyPending(0)
		normal.State.Combo = 3
		normal.applyJudgement(ctx, 0, resultPerfect)
		if normal.State.LightningActive {
			t.Fatal("normal note extended an existing combo with lightning")
		}

		chart[1] = chartObject{ID: 1, Kind: objectSpecial, TimeMS: 1000, Lane: 2}
		special := testGameWithOnlyPending(1)
		special.State.Combo = 3
		special.applyJudgement(ctx, 1, resultPerfect)
		if !special.State.LightningActive || special.State.LightningLane != 2 {
			t.Fatalf("special hit lightning = active:%t lane:%d, want active lane 2", special.State.LightningActive, special.State.LightningLane)
		}

		missed := testGameWithOnlyPending(1)
		missed.State.Combo = 3
		missed.applyJudgement(ctx, 1, resultMiss)
		if missed.State.LightningActive || missed.State.SpecialHit[1] {
			t.Fatal("missed special note triggered lightning")
		}

		chart[2] = chartObject{ID: 2, Kind: objectTap, TimeMS: 1000, Lane: 3}
		comboStart := testGameWithOnlyPending(2)
		comboStart.applyJudgement(ctx, 2, resultPerfect)
		if !comboStart.State.LightningActive || comboStart.State.LightningLane != 3 {
			t.Fatalf("combo start lightning = active:%t lane:%d, want active lane 3", comboStart.State.LightningActive, comboStart.State.LightningLane)
		}
	})
}

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
