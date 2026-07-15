package rhythm

import (
	"math"
	"testing"
)

func TestSongTimeMSProgressesThroughCountdown(t *testing.T) {
	t.Parallel()

	state := MatchState{Started: true, SongStartTick: 240}
	for _, tc := range []struct {
		tick uint64
		want int
	}{
		{tick: 0, want: -2000},
		{tick: 24, want: -1800},
		{tick: 240, want: 0},
		{tick: 360, want: 1000},
	} {
		if got := songTimeMS(state, tc.tick); got != tc.want {
			t.Errorf("songTimeMS(tick %d) = %d, want %d", tc.tick, got, tc.want)
		}
	}
}

func TestInterpolatedSongTimeMSOnlyAdvancesPresentationClock(t *testing.T) {
	t.Parallel()

	state := MatchState{Started: true, SongStartTick: 240}
	got := interpolatedSongTimeMS(state, 360, 0.5)
	want := 1000.0 + 0.5*1000/simHz
	if math.Abs(got-want) > 0.000001 {
		t.Fatalf("interpolatedSongTimeMS() = %f, want %f", got, want)
	}
}

func TestSetupEnablesRenderInterpolation(t *testing.T) {
	t.Parallel()

	game, err := NewGame()
	if err != nil {
		t.Fatal(err)
	}
	if !game.Setup().RenderInterpolation {
		t.Fatal("rhythm demo must enable render interpolation")
	}
}

func TestFireStyleGrowsAndResetsWithCombo(t *testing.T) {
	t.Parallel()

	baseScale, baseShift := fireStyle(0)
	if baseScale != 1 || baseShift != 0 {
		t.Fatalf("base fire style = scale:%f shift:%f, want 1 and 0", baseScale, baseShift)
	}
	comboScale, comboShift := fireStyle(50)
	if comboScale <= baseScale || comboShift <= baseShift {
		t.Fatalf("combo fire style = scale:%f shift:%f, want larger and shifted", comboScale, comboShift)
	}
	light := firePaletteEntry(0.75, 0)
	yellow := firePaletteEntry(0.75, comboShift)
	if light == yellow {
		t.Fatal("combo fire palette did not change color")
	}
	if yellow[0] <= yellow[2] || yellow[1] <= yellow[2] {
		t.Fatalf("high-combo fire = %#v, want a yellow tone", yellow)
	}
	maxScale, _ := fireStyle(fireMaxCombo)
	if maxHeight := fireBaseHeight * maxScale; math.Abs(maxHeight-worldH) > 0.000001 {
		t.Fatalf("maximum flame height = %f, want world height %f", maxHeight, worldH)
	}
	at85Scale, _ := fireStyle(85)
	if height := fireBaseHeight * at85Scale; height < worldH-2 {
		t.Fatalf("combo 85 flame height = %f, want within 2 units of world height %f", height, worldH)
	}
	resetScale, resetShift := fireStyle(0)
	if resetScale != baseScale || resetShift != baseShift {
		t.Fatalf("reset fire style = scale:%f shift:%f, want original", resetScale, resetShift)
	}
}

func TestTallComboKeepsHeatAliveAtTheTop(t *testing.T) {
	t.Parallel()

	fire := fireSim{}
	if got := fire.cooling(0); got != 2 {
		t.Fatalf("base cooling = %d, want 2", got)
	}
	fire.styleCombo = fireTallCombo
	if got := fire.cooling(0); got != 1 {
		t.Fatalf("tall-combo cooling = %d, want 1", got)
	}
}

func TestFireBoltsRequireComboAndGrowWithIt(t *testing.T) {
	t.Parallel()

	if commands := fireBoltCommands(0, 0, fireBaseWidth, fireBaseHeight); commands != nil {
		t.Fatal("zero combo created fire bolts")
	}
	lowScale, _ := fireStyle(fireBoltMinCombo)
	low := fireBoltCommands(fireBoltMinCombo, 0, fireBaseWidth*lowScale, fireBaseHeight*lowScale)
	if len(low) == 0 {
		t.Fatal("minimum combo did not create an occasional fire bolt pulse")
	}
	maxScale, _ := fireStyle(fireMaxCombo)
	high := fireBoltCommands(fireMaxCombo, 0, fireBaseWidth*maxScale, fireBaseHeight*maxScale)
	if len(high) <= len(low) {
		t.Fatalf("high combo bolt commands = %d, want more than low combo %d", len(high), len(low))
	}
}
