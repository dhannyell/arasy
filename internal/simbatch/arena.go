package simbatch

import (
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
	"github.com/dhannyell/arasy/scene"
)

// The arena is a closed square world with eight bouncing balls. Every
// parameter derives from the scenario index through integer arithmetic,
// so a scenario is a pure function of its index and needs no RNG.
const (
	arenaSize  = 100.0
	arenaBalls = 8
	ballRadius = 4.0
	arenaDT    = 1.0 / 60.0
)

// ArenaResult summarizes one arena scenario after it runs.
type ArenaResult struct {
	Checksum  uint64
	Contacts  int
	MeanSpeed float64
}

// ArenaRestitution returns the wall restitution that scenario i of n
// receives. The sweep is linear from 0.30 to 0.95.
func ArenaRestitution(scenario, scenarios int) float64 {
	if scenarios <= 1 {
		return 0.30
	}
	span := 0.65 * float64(scenario)
	return 0.30 + span/float64(scenarios-1)
}

// Arena builds the bounce-arena scenario i of n and steps it for ticks.
// Arena is the package's reference fixture for tests, benchmarks, and the
// CLI. It is not a game API.
func Arena(scenario, scenarios, ticks int) ArenaResult {
	world, balls := buildArena(scenario, scenarios)
	config := scene.StepConfig{
		Bounds: math2d.Rect{Max: math2d.Vec2{X: arenaSize, Y: arenaSize}},
		Bounce: scene.BounceConfig{UseBodyRestitution: true},
	}
	contacts := 0
	for range ticks {
		stats := world.Step(arenaDT, config)
		contacts += stats.Contacts
	}
	speedSum := 0.0
	for _, id := range balls {
		velocity, _ := world.Velocity(id)
		speedSum += velocity.Linear.Len()
	}
	return ArenaResult{
		Checksum:  world.Checksum(),
		Contacts:  contacts,
		MeanSpeed: speedSum / arenaBalls,
	}
}

// ArenaBall is the spawn state of one ball of a scenario.
type ArenaBall struct {
	Position math2d.Vec2
	Velocity math2d.Vec2
}

// ArenaLayout returns the spawn layout of a scenario: a 4x2 grid with
// position and velocity jitter per scenario and per ball. The jitter stays
// in integer space, so spawn values are exact in float64 and identical
// across architectures. The CLI prints the layout to reproduce a scenario
// in isolation.
func ArenaLayout(scenario int) [arenaBalls]ArenaBall {
	var layout [arenaBalls]ArenaBall
	for b := range arenaBalls {
		col := b % 4
		row := b / 4
		x := 20 + 20*col + (scenario*7+b*13)%11 - 5
		y := 33 + 34*row + (scenario*3+b*19)%11 - 5
		vx := 3 * ((scenario*5+b*17)%21 - 10)
		vy := 3 * ((scenario*11+b*3)%21 - 10)
		layout[b] = ArenaBall{
			Position: math2d.Vec2{X: float64(x), Y: float64(y)},
			Velocity: math2d.Vec2{X: float64(vx), Y: float64(vy)},
		}
	}
	return layout
}

func buildArena(scenario, scenarios int) (*scene.World, [arenaBalls]scene.EntityID) {
	world := scene.NewWorld()
	restitution := ArenaRestitution(scenario, scenarios)
	var balls [arenaBalls]scene.EntityID
	for b, ball := range ArenaLayout(scenario) {
		id := world.SpawnBundle(scene.Bundle{
			Transform: &scene.Transform{
				Position: ball.Position,
				Scale:    math2d.Vec2{X: 1, Y: 1},
			},
			CircleSprite: &scene.CircleSprite{
				Radius: ballRadius,
				BaseSprite: scene.BaseSprite{
					Color: render.Color{R: 1, G: 1, B: 1, A: 1},
				},
			},
			CircleCollider: &scene.CircleCollider{Radius: ballRadius},
			Body:           &scene.Body{InvMass: 1, Restitution: restitution},
		})
		world.SetVelocity(id, scene.Velocity{Linear: ball.Velocity})
		balls[b] = id
	}
	return world, balls
}
