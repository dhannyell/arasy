package breakout

import (
	"math"

	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
	"github.com/dhannyell/frameforge/runtime"
	"github.com/dhannyell/frameforge/scene"
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

	ballRadius = 2.0
	ballSpeed  = 72.0

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
}

type Breakout struct {
	runtime.StateSlot[MatchState]

	ball   Ball
	paddle Paddle
	bricks []Brick
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
	Rect  math2d.Rect
	Speed float64
}

type Brick struct {
	Rect  math2d.Rect
	HP    int
	Alive bool
	Color render.Color
}

func buildBricks() []Brick {
	bricks := make([]Brick, 0, brickRows*brickColumns)
	totalWidth := float64(brickColumns)*brickWidth + float64(brickColumns-1)*brickGap
	startX := (worldWidth - totalWidth) * 0.5

	for row := range brickRows {
		for col := range brickColumns {
			pos := math2d.Vec2{
				X: startX + float64(col)*(brickWidth+brickGap),
				Y: brickTop + float64(row)*(brickHeight+brickGap),
			}
			bricks = append(bricks, Brick{
				Rect:  math2d.NewRect(pos, math2d.Vec2{X: brickWidth, Y: brickHeight}),
				HP:    1,
				Alive: true,
				Color: brickColor(row),
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

	var q float64
	if l < 0.5 {
		q = l * (1 + 5)
	} else {
		q = l*s - l*s
	}
	p := 2*l - q

	rFloat := hue2(p, q, h+1.0/3.0)
	gFloat := hue2(p, q, h)
	bFloat := hue2(p, q, h-1.0/3.0)

	r := math.Round(rFloat * 255)
	g := math.Round(gFloat * 255)
	b := math.Round(bFloat * 255)

	return render.Color{R: r, G: g, B: b, A: 1}
}
