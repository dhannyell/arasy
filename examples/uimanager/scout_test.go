package uimanager

import (
	"flag"
	"fmt"
	"testing"

	"github.com/dhannyell/arasy/input"
	"github.com/dhannyell/arasy/internal/replaytest"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/runtime"
)

var update = flag.Bool("update", false, "rewrite golden fixtures")

func typeText(d *runtime.Driver, text string) {
	for _, r := range text {
		event, ok := input.TextEvent(r)
		if !ok {
			continue
		}
		d.ApplyInput([]input.Event{event})
		d.Step()
	}
}

func pressKey(d *runtime.Driver, code input.SourceCode) {
	d.ApplyInput([]input.Event{{Source: input.Keyboard(code), Kind: input.EventButtonDown}})
	d.Step()
	d.ApplyInput([]input.Event{{Source: input.Keyboard(code), Kind: input.EventButtonUp}})
	d.Step()
}

func openScout(t *testing.T) (*Manager, *runtime.Driver) {
	t.Helper()
	manager, driver := newDriver(t)
	clickNav(driver, navItems[ScreenScout].rect.Center())
	if manager.State.Screen != ScreenScout {
		t.Fatalf("screen = %v, want the scout screen", manager.State.Screen)
	}
	return manager, driver
}

func headerCenter(col int) math2d.Vec2 {
	rect := scoutTableRect()
	x := rect.Min.X
	for i, column := range scoutColumns() {
		if i == col {
			break
		}
		x += column.Width
	}
	width := scoutColumns()[col].Width
	if width == 0 {
		width = 100
	}
	return math2d.Vec2{X: x + width/2, Y: rect.Min.Y + 18}
}

func TestScoutPoolHasEnoughRowsToNeedVirtualization(t *testing.T) {
	manager, _ := newDriver(t)

	if got := len(manager.State.ScoutPool); got < 900 {
		t.Fatalf("scout pool = %d rows, want the market to be large enough to matter", got)
	}
}

// Typing filters the market, which is the whole point of routing text through
// the input vocabulary: the query is game state and the rows follow it.
func TestTypingFiltersTheMarket(t *testing.T) {
	manager, driver := openScout(t)
	before := len(manager.scoutVisibleRows())

	clickNav(driver, scoutSearchRect().Center())
	typeText(driver, "ar")

	if manager.State.ScoutQuery != "ar" {
		t.Fatalf("query = %q, want \"ar\"", manager.State.ScoutQuery)
	}
	after := len(manager.scoutVisibleRows())
	if after == 0 || after >= before {
		t.Fatalf("rows = %d, want fewer than the unfiltered %d", after, before)
	}
}

func TestBackspaceDeletesFromTheQuery(t *testing.T) {
	manager, driver := openScout(t)
	clickNav(driver, scoutSearchRect().Center())
	typeText(driver, "arm")

	pressKey(driver, input.KeyBackspace)

	if manager.State.ScoutQuery != "ar" {
		t.Fatalf("query = %q, want \"ar\" after one backspace", manager.State.ScoutQuery)
	}
}

func TestClickingHeaderSortsTheMarket(t *testing.T) {
	manager, driver := openScout(t)

	clickNav(driver, headerCenter(scoutColRating))
	if manager.State.ScoutSortBy != (ScoutSort{Col: scoutColRating}) {
		t.Fatalf("sort = %+v, want ascending by rating", manager.State.ScoutSortBy)
	}
	rows := manager.scoutVisibleRows()
	first, last := manager.State.ScoutPool[rows[0]].Rating, manager.State.ScoutPool[rows[len(rows)-1]].Rating
	if first > last {
		t.Fatalf("ascending sort put %d before %d", first, last)
	}

	clickNav(driver, headerCenter(scoutColRating))
	if !manager.State.ScoutSortBy.Desc {
		t.Fatal("clicking the same header again did not flip the direction")
	}
	rows = manager.scoutVisibleRows()
	if manager.State.ScoutPool[rows[0]].Rating < manager.State.ScoutPool[rows[len(rows)-1]].Rating {
		t.Fatal("descending sort left the rows ascending")
	}
}

func TestDropdownFiltersByPosition(t *testing.T) {
	manager, driver := openScout(t)

	clickNav(driver, scoutFilterRect().Center())
	rowH := matchdayTheme().Row.Height
	// The second option, "GOALKEEPERS", in the list hanging under the button.
	clickNav(driver, math2d.Vec2{
		X: scoutFilterRect().Center().X,
		Y: scoutFilterRect().Max.Y + rowH*1.5,
	})

	if manager.State.ScoutFilter != 1 {
		t.Fatalf("filter = %d, want the goalkeeper option", manager.State.ScoutFilter)
	}
	for _, row := range manager.scoutVisibleRows() {
		if posAbbr(manager.State.ScoutPool[row].Player.Pos) != "GK" {
			t.Fatal("the goalkeeper filter left an outfield player in the market")
		}
	}
}

func TestU23CheckboxFiltersByAge(t *testing.T) {
	manager, driver := openScout(t)

	clickNav(driver, scoutU23Rect().Center())

	if !manager.State.ScoutOnlyU23 {
		t.Fatal("clicking the checkbox did not set the filter")
	}
	for _, row := range manager.scoutVisibleRows() {
		if age := manager.State.ScoutPool[row].Player.Age; age > 23 {
			t.Fatalf("U23 filter left a %d-year-old in the market", age)
		}
	}
}

func TestKeyboardWalksTheMarketRows(t *testing.T) {
	manager, driver := openScout(t)

	// Clicking a row focuses the table and selects it; the arrows take over.
	rect := scoutTableRect()
	rowH := matchdayTheme().Row.Height
	clickNav(driver, math2d.Vec2{X: rect.Center().X, Y: rect.Min.Y + matchdayTheme().Header.Height + rowH/2})
	if manager.State.ScoutSelected != 0 {
		t.Fatalf("selected = %d, want the first row", manager.State.ScoutSelected)
	}

	pressKey(driver, input.KeyArrowDown)
	if manager.State.ScoutSelected != 1 {
		t.Fatalf("selected = %d, want the arrow to move down one row", manager.State.ScoutSelected)
	}
	pressKey(driver, input.KeyEnd)
	if want := len(manager.scoutVisibleRows()) - 1; manager.State.ScoutSelected != want {
		t.Fatalf("selected = %d, want the last row %d", manager.State.ScoutSelected, want)
	}
}

// The scout screen is all UI state that the game owns, so a rewind has to take
// the query back with everything else.
func TestScoutQuerySurvivesRewind(t *testing.T) {
	manager, driver := openScout(t)
	before := driver.State().Tick

	clickNav(driver, scoutSearchRect().Center())
	typeText(driver, "ar")
	if manager.State.ScoutQuery == "" {
		t.Fatal("typing did not reach the query")
	}

	driver.Control(runtime.ControlCommand{Kind: runtime.ControlScrubToTick, Tick: before})
	driver.Step()

	if manager.State.ScoutQuery != "" {
		t.Fatalf("query after rewind = %q, want it empty again", manager.State.ScoutQuery)
	}
}

// TestGoldenReplay drives a whole scouting session — navigate, type, correct a
// typo, sort, filter, walk rows with the keyboard — and pins it to a committed
// ARRP recording. Typed characters travel in that recording like every other
// event, so this is the proof that a session with text in it replays exactly.
func TestGoldenReplay(t *testing.T) {
	var last *Manager
	newGame := func() runtime.Game {
		last = &Manager{}
		return last
	}
	fingerprint := func(game runtime.Game) string {
		s := game.(*Manager).State
		return fmt.Sprintf("screen=%v query=%q sort=%+v filter=%d u23=%v selected=%d rows=%d",
			s.Screen, s.ScoutQuery, s.ScoutSortBy, s.ScoutFilter, s.ScoutOnlyU23, s.ScoutSelected,
			len(scoutRows(s.ScoutPool, s.ScoutQuery, s.ScoutFilter, s.ScoutOnlyU23, s.ScoutSortBy)))
	}

	script := func(d *runtime.Driver) {
		clickNav(d, navItems[ScreenScout].rect.Center())
		clickNav(d, scoutSearchRect().Center())
		typeText(d, "arx")
		pressKey(d, input.KeyBackspace)

		clickNav(d, headerCenter(scoutColRating))
		clickNav(d, headerCenter(scoutColRating))
		clickNav(d, scoutU23Rect().Center())

		rect := scoutTableRect()
		rowH := matchdayTheme().Row.Height
		clickNav(d, math2d.Vec2{X: rect.Center().X, Y: rect.Min.Y + matchdayTheme().Header.Height + rowH/2})
		pressKey(d, input.KeyArrowDown)
		pressKey(d, input.KeyArrowDown)

		if last.State.ScoutQuery != "ar" {
			t.Fatalf("script left the query at %q", last.State.ScoutQuery)
		}
	}

	replaytest.Golden(t, *update, "testdata", "scout", newGame, fingerprint, script)
}
