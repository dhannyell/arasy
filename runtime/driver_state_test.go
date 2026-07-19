package runtime

import (
	"bytes"
	"log/slog"
	"slices"
	"strings"
	"testing"

	"github.com/dhannyell/frameforge/render"
)

// statefulGame is a minimal StatefulGame built on StateSlot[int]: no
// hand-written GameState/RestoreGameState/Clone â€” GameState() and
// RestoreGameState() come promoted from the embedded StateSlot.
type statefulGame struct {
	StateSlot[int]
}

func (g *statefulGame) Setup() Config                              { return Config{HistoryLen: 4} }
func (g *statefulGame) Init(ctx *UpdateContext)                    {}
func (g *statefulGame) Update(ctx *UpdateContext, dt float64)      { g.State++ }
func (g *statefulGame) Render(ctx *RenderContext) []render.Command { return nil }

var (
	_ Game         = (*statefulGame)(nil)
	_ StatefulGame = (*statefulGame)(nil)
)

// refState carries a slice on purpose: it only snapshots correctly through
// the Cloneable hook, exercising the deep-copy path on capture AND restore.
type refState struct {
	vals []int
}

func (r refState) Clone() refState {
	return refState{vals: slices.Clone(r.vals)}
}

type statefulSliceGame struct {
	StateSlot[refState]
}

type archiveBudgetGame struct {
	StateSlot[int]
	archiveBudget int
}

func (g *archiveBudgetGame) Setup() Config {
	return Config{HistoryLen: 8, KeyframeInterval: 2, KeyframeArchiveBudget: g.archiveBudget}
}
func (g *archiveBudgetGame) Init(ctx *UpdateContext)                    {}
func (g *archiveBudgetGame) Update(ctx *UpdateContext, dt float64)      {}
func (g *archiveBudgetGame) Render(ctx *RenderContext) []render.Command { return nil }

func (g *statefulSliceGame) Setup() Config                              { return Config{HistoryLen: 4} }
func (g *statefulSliceGame) Init(ctx *UpdateContext)                    {}
func (g *statefulSliceGame) Update(ctx *UpdateContext, dt float64)      { g.State.vals[0]++ }
func (g *statefulSliceGame) Render(ctx *RenderContext) []render.Command { return nil }

func TestStateSlotRestoreClonesReferenceState(t *testing.T) {
	slot := StateSlot[refState]{State: refState{vals: []int{10}}}
	snap := slot.GameState()

	slot.RestoreGameState(snap)
	slot.State.vals[0] = 99

	slot.RestoreGameState(snap)
	if got := slot.State.vals[0]; got != 10 {
		t.Fatalf("second restore = %d, want 10 (restore must not alias the snapshot's backing array)", got)
	}
}

func TestDriverBackTwiceDoesNotCorruptSliceBackedKeyframes(t *testing.T) {
	game := &statefulSliceGame{}
	game.State = refState{vals: []int{0}}
	driver := NewDriver(game, slog.Default())

	driver.Step() // vals[0]: 0 -> 1, snapshot captured at 0
	driver.Step() // vals[0]: 1 -> 2, snapshot captured at 1

	driver.Control(ControlCommand{Kind: ControlBack})
	driver.AdvanceTime(0)
	if got := game.State.vals[0]; got != 1 {
		t.Fatalf("counter after first back = %d, want 1", got)
	}

	// Mutate after the restore: with an aliasing restore this write lands in
	// the keyframe's backing array and the second back restores garbage.
	driver.Step()

	driver.Control(ControlCommand{Kind: ControlBack})
	driver.AdvanceTime(0)
	if got := game.State.vals[0]; got != 1 {
		t.Fatalf("counter after second back = %d, want 1 (keyframe was corrupted by post-restore mutation)", got)
	}
}

func TestDriverBackRestoresGameStateInsteadOfResettingIt(t *testing.T) {
	game := &statefulGame{}
	driver := NewDriver(game, slog.Default())

	driver.Step() // State: 0 -> 1, snapshot captured at State=0
	driver.Step() // State: 1 -> 2, snapshot captured at State=1

	if game.State != 2 {
		t.Fatalf("counter = %d, want 2", game.State)
	}

	driver.Control(ControlCommand{Kind: ControlBack})
	driver.AdvanceTime(0)

	if game.State != 1 {
		t.Fatalf("counter after back = %d, want 1 (restored from snapshot, not reset to 0)", game.State)
	}
}

func TestNewDriverWarnsWhenKeyframeRestoreIsDisabled(t *testing.T) {
	var logs bytes.Buffer
	NewDriver(&inputBackGame{}, slog.New(slog.NewTextHandler(&logs, nil)))

	if got := strings.Count(logs.String(), keyframeRestoreDisabledWarning); got != 1 {
		t.Fatalf("keyframe restore warning count = %d, want 1; logs: %s", got, logs.String())
	}
}

func TestNewDriverDoesNotWarnForStatefulGame(t *testing.T) {
	var logs bytes.Buffer
	NewDriver(&statefulGame{}, slog.New(slog.NewTextHandler(&logs, nil)))

	if strings.Contains(logs.String(), keyframeRestoreDisabledWarning) {
		t.Fatalf("unexpected keyframe restore warning: %s", logs.String())
	}
}

func TestNewDriverWiresKeyframeArchiveBudget(t *testing.T) {
	for _, test := range []struct {
		name   string
		budget int
		want   int
	}{
		{name: "default", budget: 0, want: 16},
		{name: "explicit", budget: 3, want: 3},
		{name: "disabled", budget: -1, want: -1},
	} {
		t.Run(test.name, func(t *testing.T) {
			driver := NewDriver(&archiveBudgetGame{archiveBudget: test.budget}, slog.New(slog.DiscardHandler))
			if got := driver.recorder.archiveBudget; got != test.want {
				t.Fatalf("archive budget = %d, want %d", got, test.want)
			}
		})
	}
}
