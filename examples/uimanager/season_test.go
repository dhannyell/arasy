package uimanager

import "testing"

func TestStartNewSeasonDerivesSeedAndResetsSeasonState(t *testing.T) {
	manager := &Manager{}
	manager.Init(nil)

	initialSeed := manager.State.Seed
	initialYear := manager.State.SeasonYear
	for !seasonEnded(manager.State) {
		SimulateRound(&manager.State)
	}
	if len(manager.State.Results) == 0 {
		t.Fatal("expected simulated season results")
	}

	manager.startNewSeason()

	if manager.State.Seed == initialSeed {
		t.Fatalf("new season seed = %d, want a derived seed different from initial %d", manager.State.Seed, initialSeed)
	}
	if manager.State.SeasonYear != initialYear+1 {
		t.Fatalf("season year = %d, want %d", manager.State.SeasonYear, initialYear+1)
	}
	if manager.State.Round != 1 {
		t.Fatalf("round = %d, want 1", manager.State.Round)
	}
	if len(manager.State.Results) != 0 {
		t.Fatalf("results len = %d, want 0", len(manager.State.Results))
	}
}

func TestInitRestoresHardcodedInitialSeasonSeed(t *testing.T) {
	manager := &Manager{}
	manager.Init(nil)
	for !seasonEnded(manager.State) {
		SimulateRound(&manager.State)
	}
	manager.startNewSeason()
	if manager.State.Seed == initialSeasonSeed {
		t.Fatal("test setup expected derived season seed before reset")
	}

	manager.Init(nil)

	if manager.State.Seed != initialSeasonSeed {
		t.Fatalf("seed after Init = %d, want hardcoded initial seed %d", manager.State.Seed, initialSeasonSeed)
	}
	if manager.State.SeasonYear != initialSeasonYear {
		t.Fatalf("season year after Init = %d, want %d", manager.State.SeasonYear, initialSeasonYear)
	}
}
