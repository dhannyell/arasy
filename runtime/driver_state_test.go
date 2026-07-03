package runtime

import (
	"log/slog"
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
