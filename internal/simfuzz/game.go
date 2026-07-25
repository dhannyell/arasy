package simfuzz

import (
	"fmt"
	"math"

	"github.com/dhannyell/arasy/input"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
	"github.com/dhannyell/arasy/runtime"
	"github.com/dhannyell/arasy/scene"
)

// impulseScale is the per-tick velocity nudge applied from input; velocity is
// clamped so a held button cannot grow toward Inf.
const impulseScale = 2.0
const maxInputVelocity = 1e4
const rngNudgeScale = 1e-6

// fuzzBindings map the bound keys to movement actions the fuzz game reads.
var fuzzBindings = []input.Binding{
	{Source: fuzzBoundSources[0], Action: input.ActionMoveX, Scale: -1},
	{Source: fuzzBoundSources[1], Action: input.ActionMoveX, Scale: 1},
	{Source: fuzzBoundSources[2], Action: input.ActionMoveY, Scale: 1},
	{Source: fuzzBoundSources[3], Action: input.ActionMoveY, Scale: -1},
	{Source: fuzzBoundSources[4], Action: input.ActionJump, Scale: 1},
	{Source: fuzzAxisSources[0], Action: input.ActionMoveX, Scale: 1},
	{Source: fuzzAxisSources[1], Action: input.ActionMoveY, Scale: 1},
}

const (
	fuzzHistoryLen    = 16
	fuzzKeyframeEvery = 2
	fuzzArchiveBudget = 4
)

// gameState is everything the fuzz game carries across a keyframe restore.
// Anything a property asserts on after a rewind must live here: fields kept
// outside the StateSlot survive a backward seek unchanged and would be read
// as if they belonged to the restored tick. It stays a pure value type — no
// reference fields — so StateSlot needs no Clone hook.
type gameState struct {
	IDs               [MaxEntities]scene.EntityID
	Count             int
	DespawnCount      int
	FirstTransform    scene.Transform
	HasFirstTransform bool
}

// Game is the dedicated fuzz game: a minimal runtime.Game that spawns the full
// scene component vocabulary from a Script and reacts to input so the fuzzer
// exercises the input -> Driver -> World pipeline. Its fixed-size StateSlot
// makes keyframe restore safe without exposing entity iteration from scene.
type Game struct {
	runtime.StateSlot[gameState]

	script  Script
	history bool
	// scanErr is a per-tick observation of the world, not simulation state, so
	// it deliberately stays outside the StateSlot: it is written by PostStep
	// and read by the caller before the next Step.
	scanErr error
}

// NewGame builds a fuzz game for the given script.
func NewGame(s Script) *Game { return &Game{script: s} }

func newHistoryGame(s Script) *Game {
	return &Game{script: s, history: true}
}

func (g *Game) Setup() runtime.Config {
	cfg := runtime.Config{
		SimulationHz:  60,
		Bounds:        fuzzBounds,
		Physics:       scene.PhysicsConfig{CellSize: 64},
		Seed:          g.script.Seed,
		InputBindings: fuzzBindings,
	}
	if g.history {
		cfg.HistoryLen = fuzzHistoryLen
		cfg.KeyframeInterval = fuzzKeyframeEvery
		cfg.KeyframeArchiveBudget = fuzzArchiveBudget
	}
	return cfg
}

func (g *Game) Init(ctx *runtime.UpdateContext) {
	g.State = gameState{}
	g.scanErr = nil
	// Render-only: the projection lifts a sprite by its Z and drops a ground
	// shadow, which is what makes a ballistic case readable when replayed. It
	// is not part of the checksum, so it cannot move a fuzz result.
	ctx.World.SetProjection(fuzzProjection)
	ids := make([]scene.EntityID, 0, min(len(g.script.Entities), MaxEntities))
	SpawnFromScript(ctx.World, g.script, fuzzBounds, &ids)
	g.State.Count = copy(g.State.IDs[:], ids)
}

// Update drives entities from input so recorded input changes the checksum and
// replay/rewind have something non-trivial to reproduce. On a JustPressed Jump
// it despawns one tracked entity, exercising the World.Despawn path that P10
// fuzzes: despawn removes from every component store and must keep the checksum
// stable and the remaining entities free of NaN. The victim is chosen
// deterministically from the tick so two runs despawn the same entity.
func (g *Game) Update(ctx *runtime.UpdateContext, _ float64) {
	if ctx.Input.JustPressed(input.ActionJump) {
		g.despawnOne(ctx)
	}
	g.nudgeFirstVelocity(ctx)
	mx := ctx.Input.Value(input.ActionMoveX)
	my := ctx.Input.Value(input.ActionMoveY)
	if mx == 0 && my == 0 {
		return
	}
	for _, id := range g.trackedIDs() {
		vel, ok := ctx.World.Velocity(id)
		if !ok {
			continue
		}
		vel.Linear.X = clampF(vel.Linear.X+mx*impulseScale, -maxInputVelocity, maxInputVelocity)
		vel.Linear.Y = clampF(vel.Linear.Y+my*impulseScale, -maxInputVelocity, maxInputVelocity)
		ctx.World.SetVelocity(id, vel)
	}
}

func (g *Game) nudgeFirstVelocity(ctx *runtime.UpdateContext) {
	for _, id := range g.trackedIDs() {
		velocity, ok := ctx.World.Velocity(id)
		if !ok {
			continue
		}
		nudge := ctx.Rand.Float64() - 0.5
		velocity.Linear.X = float64(nudge*rngNudgeScale) + velocity.Linear.X
		ctx.World.SetVelocity(id, velocity)
		return
	}
}

// despawnOne removes the first still-alive tracked entity. Alive is checked via
// Transform presence (Despawn removes Transform). Deterministic because Init
// repopulates g.ids in script order and the alive scan is ordered.
func (g *Game) despawnOne(ctx *runtime.UpdateContext) {
	for _, id := range g.trackedIDs() {
		if _, ok := ctx.World.Transform(id); ok {
			ctx.World.Despawn(id)
			g.State.DespawnCount++
			return
		}
	}
}

// PostStep scans every live entity's components for NaN/Inf right after the
// engine steps the world. A non-nil scanErr is surfaced by checkNumericSanity.
func (g *Game) PostStep(ctx *runtime.UpdateContext, _ scene.StepStats) {
	g.scanErr = scanNaNInf(ctx.World, g.trackedIDs())
	if ids := g.trackedIDs(); len(ids) > 0 {
		g.State.FirstTransform, g.State.HasFirstTransform = ctx.World.Transform(ids[0])
	}
}

func (g *Game) trackedIDs() []scene.EntityID {
	count := clamp(g.State.Count, 0, len(g.State.IDs))
	return g.State.IDs[:count]
}

// isFinite reports whether v is neither NaN nor Inf.
func isFinite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }

func finiteVec(v math2d.Vec2) bool {
	return isFinite(v.X) && isFinite(v.Y)
}

func finiteTransform(t scene.Transform) bool {
	return finiteVec(t.Position) && isFinite(t.Rotation) && finiteVec(t.Scale)
}

func finiteColor(c render.Color) bool {
	return isFinite(c.R) && isFinite(c.G) && isFinite(c.B) && isFinite(c.A)
}

func finiteRect(r math2d.Rect) bool {
	return finiteVec(r.Min) && finiteVec(r.Max)
}

// scanNaNInf walks every tracked entity's simulation components and reports the
// first non-finite value. It is the heart of P4: a NaN born inside the pipeline
// (from extreme-but-finite input) surfaces here rather than hiding in the
// checksum's bit-stable NaN encoding.
func scanNaNInf(w *scene.World, ids []scene.EntityID) error {
	for _, id := range ids {
		if t, ok := w.Transform(id); ok {
			if !finiteTransform(t) {
				return fmt.Errorf("entity %d: non-finite transform %+v", id, t)
			}
		}
		if v, ok := w.Velocity(id); ok {
			if !finiteVec(v.Linear) {
				return fmt.Errorf("entity %d: non-finite velocity %+v", id, v)
			}
		}
		if a, ok := w.Acceleration(id); ok {
			if !finiteVec(a.Linear) {
				return fmt.Errorf("entity %d: non-finite acceleration %+v", id, a)
			}
		}
		if d, ok := w.Damping(id); ok && !isFinite(d.Linear) {
			return fmt.Errorf("entity %d: non-finite damping %+v", id, d)
		}
		if b, ok := w.Body(id); ok {
			if !isFinite(b.InvMass) || !isFinite(b.Restitution) {
				return fmt.Errorf("entity %d: non-finite body %+v", id, b)
			}
		}
		if c, ok := w.CircleCollider(id); ok && !isFinite(c.Radius) {
			return fmt.Errorf("entity %d: non-finite circle collider %+v", id, c)
		}
		if r, ok := w.RectCollider(id); ok && !finiteVec(r.HalfExtends) {
			return fmt.Errorf("entity %d: non-finite rect collider %+v", id, r)
		}
		if h, ok := w.Height(id); ok {
			if !isFinite(h.Z) || !isFinite(h.VZ) {
				return fmt.Errorf("entity %d: non-finite height %+v", id, h)
			}
		}
		if s, ok := w.Spin(id); ok {
			if !isFinite(s.Omega) {
				return fmt.Errorf("entity %d: non-finite spin %+v", id, s)
			}
		}
		if b, ok := w.Ballistics(id); ok {
			if !isFinite(b.Gravity) || !isFinite(b.DragK) || !isFinite(b.MagnusK) ||
				!isFinite(b.BounceDamping) || !isFinite(b.BounceFriction) ||
				!isFinite(b.SpinDecay) || !isFinite(b.RollFriction) || !isFinite(b.RestVZ) {
				return fmt.Errorf("entity %d: non-finite ballistics %+v", id, b)
			}
		}
		if l, ok := w.Locomotion(id); ok {
			if !isFinite(l.Accel) || !isFinite(l.Decel) || !isFinite(l.MaxSpeed) || !isFinite(l.Agility) {
				return fmt.Errorf("entity %d: non-finite locomotion %+v", id, l)
			}
		}
		if m, ok := w.MoveIntent(id); ok {
			if !finiteVec(m.Target) || !isFinite(m.DesiredSpeed) {
				return fmt.Errorf("entity %d: non-finite move intent %+v", id, m)
			}
		}
		if sprite, ok := w.CircleSprite(id); ok {
			if !isFinite(sprite.Radius) || !finiteColor(sprite.Color) {
				return fmt.Errorf("entity %d: non-finite circle sprite %+v", id, sprite)
			}
		}
		if sprite, ok := w.ImageSprite(id); ok {
			if !finiteRect(sprite.Src) || !finiteVec(sprite.Size) || !finiteColor(sprite.Tint) {
				return fmt.Errorf("entity %d: non-finite image sprite %+v", id, sprite)
			}
		}
		if tween, ok := w.TransformTween(id); ok {
			if !finiteTransform(tween.From) || !finiteTransform(tween.To) ||
				!isFinite(tween.Elapsed) || !isFinite(tween.Duration) {
				return fmt.Errorf("entity %d: non-finite transform tween %+v", id, tween)
			}
		}
		if tween, ok := w.CircleSpriteTween(id); ok {
			if !isFinite(tween.From.Radius) || !finiteColor(tween.From.Color) ||
				!isFinite(tween.To.Radius) || !finiteColor(tween.To.Color) ||
				!isFinite(tween.Elapsed) || !isFinite(tween.Duration) {
				return fmt.Errorf("entity %d: non-finite circle sprite tween %+v", id, tween)
			}
		}
	}
	return nil
}
