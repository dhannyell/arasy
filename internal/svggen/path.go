package svggen

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode"

	"github.com/dhannyell/arasy/math2d"
)

type PathVerb uint8

const (
	PathMove PathVerb = iota
	PathLine
	PathQuad
	PathCubic
	PathClose
)

type PathCommand struct {
	Verb       PathVerb
	P0, P1, P2 math2d.Vec2
}

type pathScanner struct {
	source string
	index  int
}

func ParsePathData(source string) ([]PathCommand, error) {
	scanner := pathScanner{source: source}
	var commands []PathCommand
	var current, start math2d.Vec2
	var previousCubic, previousQuad math2d.Vec2
	var previousCommand byte
	command := byte(0)
	for {
		scanner.skipSeparators()
		if scanner.index >= len(source) {
			break
		}
		if isPathCommand(source[scanner.index]) {
			command = source[scanner.index]
			scanner.index++
		} else if command == 0 || command == 'Z' || command == 'z' {
			return nil, scanner.errorf("expected path command")
		}
		relative := command >= 'a' && command <= 'z'
		upper := byte(unicode.ToUpper(rune(command)))
		readPoint := func() (math2d.Vec2, error) {
			x, err := scanner.number()
			if err != nil {
				return math2d.Vec2{}, err
			}
			y, err := scanner.number()
			if err != nil {
				return math2d.Vec2{}, err
			}
			point := math2d.Vec2{X: x, Y: y}
			if relative {
				point = point.Add(current)
			}
			return point, nil
		}
		switch upper {
		case 'M':
			point, err := readPoint()
			if err != nil {
				return nil, err
			}
			verb := PathMove
			if previousCommand == command || previousCommand == command-('m'-'M') {
				verb = PathLine
			}
			commands = append(commands, PathCommand{Verb: verb, P0: point})
			current = point
			if verb == PathMove {
				start = point
			}
			previousCommand = command
			if verb == PathMove {
				if relative {
					command = 'l'
				} else {
					command = 'L'
				}
			}
		case 'L':
			point, err := readPoint()
			if err != nil {
				return nil, err
			}
			commands = append(commands, PathCommand{Verb: PathLine, P0: point})
			current, previousCommand = point, command
		case 'H':
			value, err := scanner.number()
			if err != nil {
				return nil, err
			}
			if relative {
				value += current.X
			}
			current.X = value
			commands = append(commands, PathCommand{Verb: PathLine, P0: current})
			previousCommand = command
		case 'V':
			value, err := scanner.number()
			if err != nil {
				return nil, err
			}
			if relative {
				value += current.Y
			}
			current.Y = value
			commands = append(commands, PathCommand{Verb: PathLine, P0: current})
			previousCommand = command
		case 'C':
			control1, err := readPoint()
			if err != nil {
				return nil, err
			}
			control2, err := readPoint()
			if err != nil {
				return nil, err
			}
			to, err := readPoint()
			if err != nil {
				return nil, err
			}
			commands = append(commands, PathCommand{Verb: PathCubic, P0: control1, P1: control2, P2: to})
			previousCubic, current, previousCommand = control2, to, command
		case 'S':
			control1 := current
			if previousCommand == 'C' || previousCommand == 'c' || previousCommand == 'S' || previousCommand == 's' {
				control1 = current.Add(current.Sub(previousCubic))
			}
			control2, err := readPoint()
			if err != nil {
				return nil, err
			}
			to, err := readPoint()
			if err != nil {
				return nil, err
			}
			commands = append(commands, PathCommand{Verb: PathCubic, P0: control1, P1: control2, P2: to})
			previousCubic, current, previousCommand = control2, to, command
		case 'Q':
			control, err := readPoint()
			if err != nil {
				return nil, err
			}
			to, err := readPoint()
			if err != nil {
				return nil, err
			}
			commands = append(commands, PathCommand{Verb: PathQuad, P0: control, P1: to})
			previousQuad, current, previousCommand = control, to, command
		case 'T':
			control := current
			if previousCommand == 'Q' || previousCommand == 'q' || previousCommand == 'T' || previousCommand == 't' {
				control = current.Add(current.Sub(previousQuad))
			}
			to, err := readPoint()
			if err != nil {
				return nil, err
			}
			commands = append(commands, PathCommand{Verb: PathQuad, P0: control, P1: to})
			previousQuad, current, previousCommand = control, to, command
		case 'A':
			rx, err := scanner.number()
			if err != nil {
				return nil, err
			}
			ry, err := scanner.number()
			if err != nil {
				return nil, err
			}
			rotation, err := scanner.number()
			if err != nil {
				return nil, err
			}
			large, err := scanner.flag()
			if err != nil {
				return nil, err
			}
			sweep, err := scanner.flag()
			if err != nil {
				return nil, err
			}
			to, err := readPoint()
			if err != nil {
				return nil, err
			}
			cubics, err := arcToCubics(current, to, rx, ry, rotation, large, sweep)
			if err != nil {
				return nil, err
			}
			commands = append(commands, cubics...)
			current, previousCommand = to, command
			if len(cubics) != 0 {
				previousCubic = cubics[len(cubics)-1].P1
			}
		case 'Z':
			commands = append(commands, PathCommand{Verb: PathClose})
			current, previousCommand, command = start, command, 0
		default:
			return nil, scanner.errorf("unsupported path command %q", command)
		}
		if len(commands) > MaxPathCommands {
			return nil, fmt.Errorf("svg: path command limit %d exceeded", MaxPathCommands)
		}
	}
	if len(commands) == 0 {
		return nil, fmt.Errorf("svg: path data is empty")
	}
	return commands, nil
}

func (scanner *pathScanner) number() (float64, error) {
	scanner.skipSeparators()
	start := scanner.index
	if scanner.index < len(scanner.source) && (scanner.source[scanner.index] == '+' || scanner.source[scanner.index] == '-') {
		scanner.index++
	}
	digits := false
	for scanner.index < len(scanner.source) && scanner.source[scanner.index] >= '0' && scanner.source[scanner.index] <= '9' {
		scanner.index++
		digits = true
	}
	if scanner.index < len(scanner.source) && scanner.source[scanner.index] == '.' {
		scanner.index++
		for scanner.index < len(scanner.source) && scanner.source[scanner.index] >= '0' && scanner.source[scanner.index] <= '9' {
			scanner.index++
			digits = true
		}
	}
	if !digits {
		return 0, scanner.errorf("expected number")
	}
	if scanner.index < len(scanner.source) && (scanner.source[scanner.index] == 'e' || scanner.source[scanner.index] == 'E') {
		scanner.index++
		if scanner.index < len(scanner.source) && (scanner.source[scanner.index] == '+' || scanner.source[scanner.index] == '-') {
			scanner.index++
		}
		exponentStart := scanner.index
		for scanner.index < len(scanner.source) && scanner.source[scanner.index] >= '0' && scanner.source[scanner.index] <= '9' {
			scanner.index++
		}
		if scanner.index == exponentStart {
			return 0, scanner.errorf("invalid exponent")
		}
	}
	value, err := strconv.ParseFloat(scanner.source[start:scanner.index], 64)
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) || math.Abs(value) > MaxMagnitude {
		return 0, scanner.errorf("invalid finite number")
	}
	return value, nil
}

func (scanner *pathScanner) flag() (bool, error) {
	value, err := scanner.number()
	if err != nil || value != 0 && value != 1 {
		return false, scanner.errorf("arc flag must be 0 or 1")
	}
	return value == 1, nil
}

func (scanner *pathScanner) skipSeparators() {
	for scanner.index < len(scanner.source) {
		char := scanner.source[scanner.index]
		if char != ',' && char != ' ' && char != '\t' && char != '\r' && char != '\n' {
			break
		}
		scanner.index++
	}
}

func (scanner *pathScanner) errorf(format string, args ...any) error {
	return fmt.Errorf("svg: path offset %d: %s", scanner.index, fmt.Sprintf(format, args...))
}

func isPathCommand(char byte) bool {
	return strings.ContainsRune("MmLlHhVvCcSsQqTtAaZz", rune(char))
}

func arcToCubics(from, to math2d.Vec2, rx, ry, rotationDegrees float64, large, sweep bool) ([]PathCommand, error) {
	rx, ry = math.Abs(rx), math.Abs(ry)
	if from == to {
		return nil, nil
	}
	if rx == 0 || ry == 0 {
		return []PathCommand{{Verb: PathLine, P0: to}}, nil
	}
	phi := rotationDegrees * math.Pi / 180
	sinPhi, cosPhi := math.Sincos(phi)
	dx, dy := (from.X-to.X)*0.5, (from.Y-to.Y)*0.5
	xp := cosPhi*dx + sinPhi*dy
	yp := -sinPhi*dx + cosPhi*dy
	lambda := xp*xp/(rx*rx) + yp*yp/(ry*ry)
	if lambda > 1 {
		scale := math.Sqrt(lambda)
		rx, ry = rx*scale, ry*scale
	}
	numerator := rx*rx*ry*ry - rx*rx*yp*yp - ry*ry*xp*xp
	denominator := rx*rx*yp*yp + ry*ry*xp*xp
	factor := 0.0
	if denominator > 0 {
		factor = math.Sqrt(math.Max(0, numerator/denominator))
	}
	if large == sweep {
		factor = -factor
	}
	cxp := factor * rx * yp / ry
	cyp := factor * -ry * xp / rx
	center := math2d.Vec2{
		X: cosPhi*cxp - sinPhi*cyp + (from.X+to.X)*0.5,
		Y: sinPhi*cxp + cosPhi*cyp + (from.Y+to.Y)*0.5,
	}
	angle := func(ux, uy, vx, vy float64) float64 {
		return math.Atan2(ux*vy-uy*vx, ux*vx+uy*vy)
	}
	ux, uy := (xp-cxp)/rx, (yp-cyp)/ry
	vx, vy := (-xp-cxp)/rx, (-yp-cyp)/ry
	start := angle(1, 0, ux, uy)
	delta := angle(ux, uy, vx, vy)
	if !sweep && delta > 0 {
		delta -= 2 * math.Pi
	} else if sweep && delta < 0 {
		delta += 2 * math.Pi
	}
	count := int(math.Ceil(math.Abs(delta) / (math.Pi / 2)))
	step := delta / float64(count)
	mapPoint := func(x, y float64) math2d.Vec2 {
		return math2d.Vec2{
			X: center.X + cosPhi*rx*x - sinPhi*ry*y,
			Y: center.Y + sinPhi*rx*x + cosPhi*ry*y,
		}
	}
	result := make([]PathCommand, 0, count)
	for index := range count {
		a0 := start + float64(index)*step
		a1 := a0 + step
		k := 4.0 / 3.0 * math.Tan((a1-a0)/4)
		s0, c0 := math.Sincos(a0)
		s1, c1 := math.Sincos(a1)
		result = append(result, PathCommand{
			Verb: PathCubic,
			P0:   mapPoint(c0-k*s0, s0+k*c0),
			P1:   mapPoint(c1+k*s1, s1-k*c1),
			P2:   mapPoint(c1, s1),
		})
	}
	result[len(result)-1].P2 = to
	return result, nil
}
