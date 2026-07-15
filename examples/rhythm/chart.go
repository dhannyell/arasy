package rhythm

import "github.com/dhannyell/frameforge/math2d"

type objectKind uint8

const (
	objectTap objectKind = iota
	objectHold
	objectSpecial
	objectCircle
)

type chartObject struct {
	ID     int
	Kind   objectKind
	TimeMS int
	EndMS  int
	Lane   int
	Pos    math2d.Vec2
	Radius float64
}
