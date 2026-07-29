package rhythm

import (
	"embed"
	"io/fs"

	"github.com/dhannyell/arasy/asset"
	"github.com/dhannyell/arasy/audio"
	"github.com/dhannyell/arasy/input"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
	"github.com/dhannyell/arasy/runtime"
)

//go:embed assets
var embeddedAssets embed.FS

type phase uint8

const (
	phaseMenu phase = iota
	phaseCountdown
	phasePlaying
	phaseResults
)

type objectResult uint8

const (
	resultPending objectResult = iota
	resultMarvelous
	resultPerfect
	resultGood
	resultBad
	resultMiss
	resultHeld
)

const (
	simHz             = 120
	countInMS         = 2000
	approachMS        = 1800.0
	despawnAfterMS    = 220.0
	calibrationStepMS = 5
)

type MatchState struct {
	Phase phase

	SongStartTick uint64
	Started       bool

	CalibrationMS    int
	Score            int
	Combo            int
	MaxCombo         int
	Health           int
	Marvelous        int
	Perfect          int
	Good             int
	Bad              int
	Miss             int
	LastResult       objectResult
	LastResultMS     int
	LaneDown         [laneCount]bool
	LaneFlashUntilMS [laneCount]int
	LightningActive  bool
	LightningLane    int
	LightningUntilMS int
	SpecialHit       [maxObjects]bool

	Results [maxObjects]objectResult
}

type Game struct {
	runtime.StateSlot[MatchState]
	assets asset.Manifest
	best   int
	fire   *fireSim // cosmetic state advanced only by Update
}

func NewGame() (*Game, error) {
	sub, err := fs.Sub(embeddedAssets, "assets")
	if err != nil {
		return nil, err
	}
	m, err := asset.Load(sub)
	if err != nil {
		return nil, err
	}
	return &Game{assets: m, fire: newFireSim()}, nil
}

func (g *Game) Setup() runtime.Config {
	return runtime.Config{
		Camera: render.Camera{
			World:    math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: worldW, Y: worldH}),
			Viewport: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 1280, Y: 960}),
			Aspect:   render.AspectExpand,
		},
		Clear:               render.RGBA(0.025, 0.025, 0.03, 1),
		SimulationHz:        simHz,
		FPS:                 60,
		RenderInterpolation: true,
		InputBindings:       bindings(),
		HistoryLen:          simHz * 90,
		KeyframeInterval:    30,
		Seed:                0x053_20260708,
		Assets:              g.assets,
	}
}

func (g *Game) Init(ctx *runtime.UpdateContext) {
	g.State = MatchState{Phase: phaseMenu, Health: 1000}
}

func (g *Game) Update(ctx *runtime.UpdateContext, dt float64) {
	ctx.HoldSimulation()

	g.handleInputs(ctx)
	g.fire.update(dt, g.State.Combo)
}

func (g *Game) Render(ctx *runtime.RenderContext) []render.Command {
	nowMS := songTimeMS(g.State, ctx.Tick)
	renderNowMS := interpolatedSongTimeMS(g.State, ctx.Tick, ctx.Alpha)

	cmds := drawBackground(ctx)
	cmds = append(cmds, g.drawFire(ctx, g.State.Combo)...)
	switch g.State.Phase {
	case phaseCountdown, phasePlaying:
		cmds = append(cmds, drawActiveHoldLanes(g.State)...)
		// Gameplay continues to judge against whole fixed ticks. Only the
		// presentation clock advances through the remainder of the tick, so
		// fast notes move smoothly without changing input timing or replay.
		cmds = append(cmds, drawChart(ctx, g.State, renderNowMS)...)
		cmds = append(cmds, drawHitTargets(ctx, g.State, nowMS)...)
		cmds = append(cmds, drawLightning(ctx, g.State, renderNowMS)...)
		cmds = append(cmds, drawHUD(g.State, nowMS)...)
		if g.State.Phase == phaseCountdown {
			cmds = append(cmds, drawCountdown(nowMS)...)
		}
	case phaseResults:
		cmds = append(cmds, drawHUD(g.State, nowMS)...)
		cmds = append(cmds, drawResults(g.State, g.best)...)
	default:
		cmds = append(cmds, drawMenu()...)
	}
	return cmds
}

// interpolatedSongTimeMS is the presentation-only song clock. Alpha is the
// unconsumed fraction of the current fixed simulation tick and is supplied by
// runtime only when RenderInterpolation is enabled.
func interpolatedSongTimeMS(state MatchState, tick uint64, alpha float64) float64 {
	return float64(songTimeMS(state, tick)) + alpha*1000/simHz
}

func (g *Game) start(ctx *runtime.UpdateContext) {
	g.State = MatchState{
		Phase:         phaseCountdown,
		SongStartTick: ctx.Tick + uint64(countInMS*simHz/1000),
		Started:       true,
		Health:        1000,
		CalibrationMS: g.State.CalibrationMS,
	}
	ctx.Audio.PlayWith("start", audio.CueOptions{Gain: 0.7})
	ctx.Audio.SetTrack("music", audio.Track{
		Clip:      "song",
		Gain:      0.40,
		FadeSec:   0.05,
		StartTick: g.State.SongStartTick,
	})
}

func (g *Game) finish(ctx *runtime.UpdateContext) {
	g.State.Phase = phaseResults
	ctx.Audio.StopTrack("music")
}

func (g *Game) handleInputs(ctx *runtime.UpdateContext) {
	if ctx.Input.JustPressed(actionOffsetDown) {
		g.State.CalibrationMS -= calibrationStepMS
	}
	if ctx.Input.JustPressed(actionOffsetUp) {
		g.State.CalibrationMS += calibrationStepMS
	}

	switch g.State.Phase {
	case phaseMenu:
		if ctx.Input.JustPressed(input.ActionConfirm) {
			g.start(ctx)
		}
	case phaseCountdown:
		if ctx.Tick >= g.State.SongStartTick {
			g.State.Phase = phasePlaying
		}
	case phasePlaying:
		nowMS := songTimeMS(g.State, ctx.Tick)
		g.handleLaneInput(ctx, nowMS)
		g.finishHolds(ctx, nowMS)
		g.updateMisses(ctx, nowMS)
		if nowMS > chartEndMS()+2500 || g.State.Health <= 0 {
			g.finish(ctx)
		}
	case phaseResults:
		if ctx.Input.JustPressed(input.ActionConfirm) {
			cal := g.State.CalibrationMS
			g.State = MatchState{Phase: phaseMenu, CalibrationMS: cal, Health: 1000}
		}
	default:
		return
	}
}

func songTimeMS(s MatchState, tick uint64) int {
	if !s.Started {
		return -approachMS
	}
	// The countdown advances continuously toward zero, so notes can travel
	// down the highway before the music starts instead of appearing at once.
	deltaTicks := int64(tick) - int64(s.SongStartTick)
	return int(deltaTicks*1000/int64(simHz)) + s.CalibrationMS
}

const (
	actionLane0 input.Action = 32 + iota
	actionLane1
	actionLane2
	actionLane3
	actionOffsetDown
	actionOffsetUp
)

func bindings() []input.Binding {
	return []input.Binding{
		{Source: input.Keyboard(input.KeyA), Action: actionLane0, Scale: 1},
		{Source: input.Keyboard(input.KeyS), Action: actionLane1, Scale: 1},
		{Source: input.Keyboard(input.KeyJ), Action: actionLane2, Scale: 1},
		{Source: input.Keyboard(input.KeyK), Action: actionLane3, Scale: 1},
		{Source: input.Keyboard(input.KeySpace), Action: input.ActionConfirm, Scale: 1},
		{Source: input.Keyboard(input.KeyArrowLeft), Action: actionOffsetDown, Scale: 1},
		{Source: input.Keyboard(input.KeyArrowRight), Action: actionOffsetUp, Scale: 1},
	}
}
