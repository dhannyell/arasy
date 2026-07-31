package uimanager

import (
	"sort"
	"strings"

	"github.com/dhannyell/arasy/runtime"
)

// The scout screen is the demo's dense-data screen: a market of roughly a
// thousand players searched by name, filtered by position, sorted by any
// column and walked with the keyboard. Everything the widgets report — the
// query, the ordering, the filter, the selected row — lives in ManagerState,
// so the whole screen rewinds with the simulation and a recorded session
// replays the typing character by character.

const (
	freeAgentPool  = 620
	scoutColName   = 0
	scoutColClub   = 1
	scoutColPos    = 2
	scoutColAge    = 3
	scoutColRating = 4
)

// ScoutEntry is one row of the market: a player plus where they come from.
type ScoutEntry struct {
	Player Player
	Club   string
	Rating int
}

// scoutFilters are the position groups the dropdown offers, in its order.
var scoutFilters = []string{"ALL", "GOALKEEPERS", "DEFENDERS", "MIDFIELDERS", "FORWARDS"}

// buildScoutPool lists every player under contract plus a generated pool of
// free agents, so the table has enough rows for virtualization to matter.
func buildScoutPool(seed uint64, clubs []Club) []ScoutEntry {
	pool := make([]ScoutEntry, 0, len(clubs)*20+freeAgentPool)
	for _, club := range clubs {
		for _, player := range club.Squad {
			pool = append(pool, ScoutEntry{Player: player, Club: club.Short, Rating: overallRating(player)})
		}
	}

	rng := runtime.NewRand(seed).Derive("scout-market")
	for range freeAgentPool {
		player := generateFreeAgent(&rng)
		pool = append(pool, ScoutEntry{Player: player, Club: "FREE", Rating: overallRating(player)})
	}
	return pool
}

func generateFreeAgent(rng *runtime.Rand) Player {
	positions := []Position{GK, CB, LB, RB, DMF, CMF, AMF, LMF, RMF, CF, LWF, RWF, SS}
	pos := positions[rng.IntN(len(positions))]
	return Player{
		Name:    playerNames[rng.IntN(len(playerNames))],
		Pos:     pos,
		Age:     17 + rng.IntN(20),
		Attack:  rating(rng, 58, 18),
		Defense: rating(rng, 58, 18),
		Stamina: rating(rng, 62, 16),
		Form:    0.30 + rng.Float64()*0.60,
	}
}

// scoutRows applies the query, the position filter and the ordering the player
// asked for. The widgets never touch the data: they report a header click and
// a typed character, and this is where the game decides what that means.
func scoutRows(pool []ScoutEntry, query string, filter int, onlyYoung bool, sort ScoutSort) []int {
	rows := make([]int, 0, len(pool))
	needle := strings.ToUpper(strings.TrimSpace(query))
	for i, entry := range pool {
		if needle != "" && !strings.Contains(strings.ToUpper(entry.Player.Name), needle) {
			continue
		}
		if !matchesPositionFilter(entry.Player.Pos, filter) {
			continue
		}
		if onlyYoung && entry.Player.Age > 23 {
			continue
		}
		rows = append(rows, i)
	}
	sortScoutRows(pool, rows, sort)
	return rows
}

func matchesPositionFilter(pos Position, filter int) bool {
	if filter <= 0 || filter >= len(scoutFilters) {
		return true
	}
	// The filter list follows the position groups: GK, DF, MF, AT.
	return posGroup(pos) == filter-1
}

// ScoutSort is the column and direction the market is ordered by.
type ScoutSort struct {
	Col  int
	Desc bool
}

// sortScoutRows orders the visible rows. It is a stable sort keyed by the pool
// index as a tiebreak, so equal ratings keep one fixed order and a replay
// reproduces the same table.
func sortScoutRows(pool []ScoutEntry, rows []int, order ScoutSort) {
	less := func(a, b int) bool {
		left, right := pool[a], pool[b]
		switch order.Col {
		case scoutColClub:
			if left.Club != right.Club {
				return left.Club < right.Club
			}
		case scoutColPos:
			if left.Player.Pos != right.Player.Pos {
				return left.Player.Pos < right.Player.Pos
			}
		case scoutColAge:
			if left.Player.Age != right.Player.Age {
				return left.Player.Age < right.Player.Age
			}
		case scoutColRating:
			if left.Rating != right.Rating {
				return left.Rating < right.Rating
			}
		default:
			if left.Player.Name != right.Player.Name {
				return left.Player.Name < right.Player.Name
			}
		}
		return a < b
	}

	sort.SliceStable(rows, func(i, j int) bool {
		if order.Desc {
			return less(rows[j], rows[i])
		}
		return less(rows[i], rows[j])
	})
}

// toggleScoutSort is what a header click means here: the same column flips
// direction, a different column starts ascending.
func toggleScoutSort(current ScoutSort, col int) ScoutSort {
	if current.Col == col {
		return ScoutSort{Col: col, Desc: !current.Desc}
	}
	return ScoutSort{Col: col}
}
