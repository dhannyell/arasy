package uimanager

import (
	"slices"

	"github.com/dhannyell/arasy/render"
)

type Screen uint8

const (
	ScreenDashboard Screen = iota
	ScreenTable
	ScreenSquad
	ScreenCalendar
	ScreenScout
)

type ManagerState struct {
	Screen         Screen
	PreviousScreen Screen
	Seed           uint64
	SeasonYear     int
	Round          int
	Clubs          []Club
	Fixtures       []Fixture
	Results        []Result
	SelectedPlayer int

	// The scout screen's query, ordering, filters and selection are game
	// state, not widget state: they are what the player asked to see. Keeping
	// them here is what lets the screen rewind with the simulation and a
	// recorded session replay the typing.
	ScoutPool     []ScoutEntry
	ScoutQuery    string
	ScoutSortBy   ScoutSort
	ScoutFilter   int
	ScoutOnlyU23  bool
	ScoutSelected int
}

type ManagerUIState struct {
	SelectedNavItem int
	NavPulseTick    uint64
}

func (s ManagerState) Clone() ManagerState {
	s.Clubs = slices.Clone(s.Clubs)
	for i := range s.Clubs {
		s.Clubs[i].Squad = slices.Clone(s.Clubs[i].Squad)
	}
	s.Fixtures = slices.Clone(s.Fixtures)
	s.Results = slices.Clone(s.Results)
	s.ScoutPool = slices.Clone(s.ScoutPool)
	return s
}

type Club struct {
	Name, Short, Stadium string
	Accent               render.Color
	Strength             float64
	Squad                []Player
}

type Player struct {
	Name    string
	Pos     Position
	Age     int
	Attack  int
	Defense int
	Stamina int
	Form    float64
}

type Position uint8

const (
	GK Position = iota
	CB
	LB
	RB
	DMF
	CMF
	AMF
	LMF
	RMF
	SS
	CF
	LWF
	RWF
)

type Fixture struct{ Round, Home, Away int }
type Result struct {
	Fixture
	HomeGoals, AwayGoals int
}

type TableRow struct {
	Club              int
	Played            int
	Wins, Draws, Loss int
	GoalsFor          int
	GoalsAgainst      int
	Points            int
}

func (r TableRow) GoalDiff() int { return r.GoalsFor - r.GoalsAgainst }
