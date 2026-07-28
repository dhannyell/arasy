package svggen

import (
	"fmt"
	"math"
	"strings"

	"github.com/dhannyell/arasy/math2d"
)

func ParseCSSTransform(source string) (math2d.Affine, error) {
	source = strings.TrimSpace(source)
	if source == "" || source == "none" {
		return math2d.Identity(), nil
	}
	result := math2d.Identity()
	for source != "" {
		open := strings.IndexByte(source, '(')
		if open <= 0 {
			return math2d.Affine{}, fmt.Errorf("svg: malformed transform %q", source)
		}
		close := strings.IndexByte(source[open+1:], ')')
		if close < 0 {
			return math2d.Affine{}, fmt.Errorf("svg: unterminated transform")
		}
		close += open + 1
		name := strings.TrimSpace(source[:open])
		arguments := strings.TrimSpace(source[open+1 : close])
		values, err := transformNumbers(arguments)
		if err != nil {
			return math2d.Affine{}, fmt.Errorf("svg: transform %s: %w", name, err)
		}
		var transform math2d.Affine
		switch name {
		case "translate":
			if len(values) < 1 || len(values) > 2 {
				return math2d.Affine{}, fmt.Errorf("svg: translate expects one or two values")
			}
			y := 0.0
			if len(values) == 2 {
				y = values[1]
			}
			transform = math2d.Translate(math2d.Vec2{X: values[0], Y: y})
		case "scale":
			if len(values) < 1 || len(values) > 2 {
				return math2d.Affine{}, fmt.Errorf("svg: scale expects one or two values")
			}
			y := values[0]
			if len(values) == 2 {
				y = values[1]
			}
			transform = math2d.Scale(math2d.Vec2{X: values[0], Y: y})
		case "rotate":
			if len(values) != 1 && len(values) != 3 {
				return math2d.Affine{}, fmt.Errorf("svg: rotate expects angle and optional center")
			}
			transform = math2d.Rotate(values[0] * math.Pi / 180)
			if len(values) == 3 {
				transform = transform.About(math2d.Vec2{X: values[1], Y: values[2]})
			}
		case "matrix":
			if len(values) != 6 {
				return math2d.Affine{}, fmt.Errorf("svg: matrix expects six values")
			}
			transform = math2d.Affine{A: values[0], B: values[1], C: values[2], D: values[3], E: values[4], F: values[5]}
		default:
			return math2d.Affine{}, fmt.Errorf("svg: transform %s is outside the supported subset", name)
		}
		result = result.Mul(transform)
		source = strings.TrimSpace(source[close+1:])
	}
	return result, nil
}

func transformNumbers(source string) ([]float64, error) {
	fields := strings.FieldsFunc(source, func(char rune) bool {
		return char == ',' || char == ' ' || char == '\t' || char == '\r' || char == '\n'
	})
	values := make([]float64, 0, len(fields))
	for _, field := range fields {
		field = strings.TrimSuffix(strings.TrimSuffix(strings.TrimSuffix(field, "px"), "deg"), "rad")
		list, err := parseNumberList(field)
		if err != nil || len(list) != 1 {
			return nil, fmt.Errorf("invalid transform number %q", field)
		}
		values = append(values, list[0])
	}
	return values, nil
}
