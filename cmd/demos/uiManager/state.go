package uiManager

import (
	"fmt"
	"slices"

	"github.com/dhannyell/frameforge/render"
)

type Screen uint8

const (
	ScreenDashboard Screen = iota
	ScreenTable
	ScreenSquad
	ScreenCalendar
)

type ManagerState struct {
	Screen         Screen
	PreviousScreen Screen
	Seed           uint64
	Round          int
	Clubs          []Club
	Fixtures       []Fixture
	Results        []Result
	SelectedPlayer int
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
func (r TableRow) Cell(c int, clubs []Club) string {
	switch c {
	case 0:
		return ""
	case 1:
		return clubs[r.Club].Name
	case 2:
		return fmt.Sprintf("%d", r.Points)
	case 3:
		return fmt.Sprintf("%d", r.Played)
	case 4:
		return fmt.Sprintf("%d", r.Wins)
	case 5:
		return fmt.Sprintf("%d", r.Draws)
	case 6:
		return fmt.Sprintf("%d", r.Loss)
	case 7:
		gd := r.GoalDiff()
		switch {
		case gd > 0:
			return fmt.Sprintf("+%d", gd)
		default:
			return fmt.Sprintf("%d", gd)
		}
	default:
		return ""
	}
}
