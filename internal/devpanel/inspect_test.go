package devpanel

import (
	"strings"
	"testing"
	"time"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
	"github.com/dhannyell/arasy/runtime"
	"github.com/dhannyell/arasy/scene"
)

// inspectGame carries a StateSlot like every real game (the inspector reads
// its state through it) and spawns two entities for the world side.
type inspectGame struct {
	runtime.StateSlot[inspectMatchState]
	ball    scene.EntityID
	keeper  scene.EntityID
	updates int
}

type inspectMatchState struct {
	Score int
	Combo float64
}

func (g *inspectGame) Setup() runtime.Config {
	cfg := testGame{}.Setup()
	// The zero BounceConfig clamps every moving collider into a zero rect,
	// so open the sides: the ball must keep the position the tests assert.
	cfg.Bounds = math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 800, Y: 600})
	cfg.Bounce = scene.BounceConfig{
		OpenSides: scene.SideLeft | scene.SideRight | scene.SideTop | scene.SideBottom,
	}
	return cfg
}
func (g *inspectGame) Init(ctx *runtime.UpdateContext) {
	g.State.Score = 7
	g.State.Combo = 2.5

	g.ball = ctx.World.Spawn()
	ctx.World.SetTransform(g.ball, scene.Transform{Position: math2d.Vec2{X: 3, Y: 4}})
	ctx.World.SetVelocity(g.ball, scene.Velocity{Linear: math2d.Vec2{X: 1}})
	ctx.World.SetCircleCollider(g.ball, scene.CircleCollider{Radius: 5})

	g.keeper = ctx.World.Spawn()
	ctx.World.SetTransform(g.keeper, scene.Transform{})
	ctx.World.SetTransformTween(g.keeper, scene.TransformTween{Duration: 100})
}
func (g *inspectGame) Update(ctx *runtime.UpdateContext, _ float64) {
	g.updates++
	if g.updates == 3 {
		ctx.World.Despawn(g.ball) // the ball leaves the world mid-run
	}
}
func (g *inspectGame) Render(*runtime.RenderContext) []render.Command { return nil }

func newInspectPanel(t *testing.T) *Panel {
	t.Helper()
	driver := runtime.NewDriver(&inspectGame{}, quietLog())
	driver.SkipSplash()
	driver.SetViewport(testViewport())
	driver.Step() // runs Init: state and entities exist from here on
	p := New(driver, Options{Log: quietLog()})
	p.SetVisible(true)
	p.activeTab = tabInspect
	return p
}

// renderTab drives one frame of the open tab: the interaction pass with no
// new events, then the draw pass, and returns what was drawn.
func renderTab(p *Panel) []render.Command {
	p.HandleInput(nil, testViewport())
	return p.Render(frameCamera(), 16*time.Millisecond, Stats{})
}

func requireText(t *testing.T, cmds []render.Command, want string) {
	t.Helper()
	if !containsText(cmds, want) {
		t.Fatalf("panel did not draw %q", want)
	}
}

// requireTextContains matches a drawn text by substring: live values (a
// position the sim already integrated) and indented paths have no stable
// exact form, but their core does.
func requireTextContains(t *testing.T, cmds []render.Command, sub string) {
	t.Helper()
	for _, cmd := range cmds {
		if text, ok := cmd.(render.DrawText); ok && strings.Contains(text.Text, sub) {
			return
		}
	}
	t.Fatalf("panel drew no text containing %q", sub)
}

func TestInspectGameStateReadsThroughTheStateSlot(t *testing.T) {
	p := newInspectPanel(t)

	cmds := renderTab(p)
	requireText(t, cmds, "Score: 7")
	requireText(t, cmds, "Combo: 2.5")
}

func TestInspectGameStateWithoutStatefulGame(t *testing.T) {
	p := openPanel(t, Options{}, tabInspect)

	cmds := renderTab(p)
	requireText(t, cmds, "(the game does not implement StatefulGame)")
}

func TestInspectEntitiesModeListsAndSelects(t *testing.T) {
	p := newInspectPanel(t)
	lay := inspectRects(p.frameLayout(testViewport()).body, inspectModeGame, false)

	clickPanel(p, lay.modes[1].Center()) // Entities

	entities := p.driver.WorldView().Entities()
	if len(entities) != 2 {
		t.Fatalf("world holds %v, want the two spawned entities", entities)
	}
	cmds := renderTab(p)
	requireText(t, cmds, "#"+formatUint(uint64(entities[0])))
	requireText(t, cmds, "#"+formatUint(uint64(entities[1])))

	// Select the first row: the ball, with a transform worth reading.
	worldLay := inspectRects(p.frameLayout(testViewport()).body, inspectModeWorld, false)
	clickPanel(p, math2d.Vec2{
		X: worldLay.entities.Min.X + 8,
		Y: worldLay.entities.Min.Y + inspectRowHeight/2,
	})

	cmds = renderTab(p)
	requireText(t, cmds, "Transform")
	requireTextContains(t, cmds, "Position.X: 3")
	requireTextContains(t, cmds, "Position.Y: 4")
	requireText(t, cmds, "Velocity")
}

func TestInspectSelectionFollowsTheEntityOutOfTheWorld(t *testing.T) {
	p := newInspectPanel(t)
	entities := p.driver.WorldView().Entities()
	lay := inspectRects(p.frameLayout(testViewport()).body, inspectModeGame, false)
	clickPanel(p, lay.modes[1].Center())

	worldLay := inspectRects(p.frameLayout(testViewport()).body, inspectModeWorld, false)
	clickPanel(p, math2d.Vec2{
		X: worldLay.entities.Min.X + 8,
		Y: worldLay.entities.Min.Y + inspectRowHeight/2,
	})
	if cmds := renderTab(p); !containsText(cmds, "Transform") {
		t.Fatal("selection did not take")
	}

	p.driver.Step()
	p.driver.Step() // third Update: the ball despawns

	cmds := renderTab(p)
	requireText(t, cmds, "#"+formatUint(uint64(entities[0]))+" left the world")
}
