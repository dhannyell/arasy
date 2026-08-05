package breakout

import (
	"math"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
	"github.com/dhannyell/arasy/runtime"
	"github.com/dhannyell/arasy/scene"
)

const (
	viewportWidth  = 1280
	viewportHeight = 960

	worldWidth  = 160.0
	worldHeight = 120.0

	paddleWidth  = 24.0
	paddleHeight = 4.0
	paddleY      = 108.0
	paddleSpeed  = 90.0

	ballRadius          = 2.0
	ballSpeed           = 72.0
	ballSpeedUpPerBrick = 1.015
	ballMaxSpeed        = 110.0

	brickRows    = 6
	brickColumns = 12
	brickWidth   = 10.0
	brickHeight  = 4.0
	brickGap     = 1.5
	brickTop     = 14.0

	physicsCellSize = 8.0
)

type MatchState struct {
	score      int
	lives      int
	bricksLeft int
	state      State

	ball   Ball
	paddle Paddle
	bricks []Brick
}

// Clone deep-copies bricks so rewind snapshots don't share the backing
// array with live state (see runtime.Cloneable).
func (m MatchState) Clone() MatchState {
	clone := m
	clone.bricks = append([]Brick(nil), m.bricks...)
	return clone
}

type Breakout struct {
	runtime.StateSlot[MatchState]
}

type State int

const (
	StateMenu State = iota
	StateReady
	StatePlaying
	StateWon
	StateGameOver
)

type Ball struct {
	id       scene.EntityID
	Position math2d.Vec2
	Velocity math2d.Vec2
	Radius   float64
}

type Paddle struct {
	id    scene.EntityID
	Speed float64
	Rect  math2d.Rect
}

type Brick struct {
	ID    scene.EntityID
	HP    int
	Alive bool
}

func spawnBuildBricks(world *scene.World) []Brick {
	bricks := make([]Brick, 0, brickRows*brickColumns)
	totalWidth := float64(brickColumns)*brickWidth + float64(brickColumns-1)*brickGap
	startX := (worldWidth - totalWidth) * 0.5

	for row := range brickRows {
		for col := range brickColumns {
			pos := math2d.Vec2{
				X: startX + float64(col)*(brickWidth+brickGap),
				Y: brickTop + float64(row)*(brickHeight+brickGap),
			}
			center := pos.Add(math2d.Vec2{X: brickWidth / 2, Y: brickHeight / 2})
			id := world.SpawnBundle(
				scene.Bundle{
					Transform: &scene.Transform{
						Position: center,
					},
					RectCollider: &scene.RectCollider{
						HalfExtends: math2d.Vec2{X: brickWidth / 2, Y: brickHeight / 2},
					},
					RectSprite: &scene.RectSprite{
						BaseSprite: scene.BaseSprite{
							Layer: 1,
							Color: brickColor(row),
						},
						Rect: math2d.NewRectFromCorners(
							math2d.Vec2{X: -brickWidth / 2, Y: -brickHeight / 2},
							math2d.Vec2{X: brickWidth / 2, Y: brickHeight / 2},
						),
					},
					Body: &scene.Body{
						InvMass:     0,
						Restitution: 1,
					},
				})

			bricks = append(bricks, Brick{
				ID:    id,
				HP:    1,
				Alive: true,
			})
		}
	}
	return bricks
}

func brickColor(row int) render.Color {
	const aurea = 0.618033988749895
	s := 0.75
	l := 0.50

	h := math.Mod(float64(row)*aurea, 1.0)

	hue2 := func(p, q, t float64) float64 {
		if t < 0 {
			t += 1
		}
		if t > 1 {
			t -= 1
		}
		if t < 1.0/6.0 {
			return p + (q-p)*6*t
		}
		if t < 1.0/2.0 {
			return q
		}
		if t < 2.0/3.0 {
			return p + (q-p)*(2.0/3.0-t)*6
		}
		return p
	}

	q := l + s - l*s
	p := 2*l - q

	rFloat := hue2(p, q, h+1.0/3.0)
	gFloat := hue2(p, q, h)
	bFloat := hue2(p, q, h-1.0/3.0)

	return render.Color{R: rFloat, G: gFloat, B: bFloat, A: 1}
}
