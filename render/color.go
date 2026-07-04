package render

import (
	"errors"
	"strconv"
	"strings"
)

// Color represents an RGBA color using normalized values from 0 to 1.
type Color struct {
	R, G, B, A float64
}

func (c *Color) Lerp(c2 Color, t float64) Color {
	return Color{
		R: c.R + (c2.R-c.R)*t,
		G: c.G + (c2.G-c.G)*t,
		B: c.B + (c2.B-c.B)*t,
		A: c.A + (c2.A-c.A)*t,
	}
}

// RGBA creates a color from normalized red, green, blue and alpha values.
func RGBA(r, g, b, a float64) Color {
	return Color{
		R: r,
		G: g,
		B: b,
		A: a,
	}
}

func RGBA8(r, g, b, a uint8) Color {
	return Color{
		R: float64(r) / 255.0,
		G: float64(g) / 255.0,
		B: float64(b / 255.0),
		A: float64(a / 255.0),
	}
}

// ColorFromHex returns a color from a hexadecimal string.
//
// Supported formats are RGB, RGBA, RRGGBB and RRGGBBAA, with or without a
// leading "#". Returned channel values are normalized from 0 to 1.
func ColorFromHex(hex string) (Color, error) {
	hex = strings.TrimPrefix(strings.TrimSpace(hex), "#")

	length := len(hex)

	if length == 3 || length == 4 {
		var expanded strings.Builder
		for _, char := range hex {
			expanded.WriteString(string(char) + string(char))
		}
		hex = expanded.String()
		length = len(hex)
	}

	if length != 6 && length != 8 {
		return Color{}, errors.New("invalid hexadecimal string")
	}

	r, err := strconv.ParseUint(hex[0:2], 16, 8)
	if err != nil {
		return Color{}, errors.New("error when parsing red channel")
	}

	g, err := strconv.ParseUint(hex[2:4], 16, 8)
	if err != nil {
		return Color{}, errors.New("error when parsing green channel")
	}

	b, err := strconv.ParseUint(hex[4:6], 16, 8)
	if err != nil {
		return Color{}, errors.New("error when parsing blue channel")
	}

	alpha := 1.0
	if length == 8 {
		a, err := strconv.ParseUint(hex[6:8], 16, 8)
		if err != nil {
			return Color{}, errors.New("error when parsing alpha channel")
		}
		alpha = float64(a) / 255.0
	}

	return Color{
		R: float64(r) / 255.0,
		G: float64(g) / 255.0,
		B: float64(b) / 255.0,
		A: alpha,
	}, nil
}
