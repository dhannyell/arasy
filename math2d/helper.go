package math2d

import "math"

func Clamp(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func ClampRect(rect Rect, min, max float64) Rect {
	if rect.Min.X < min {
		shift := min - rect.Min.X
		rect.Min.X += shift
		rect.Max.X += shift
	}
	if rect.Max.X > max {
		shift := rect.Max.X - max
		rect.Min.X -= shift
		rect.Max.X -= shift
	}
	return rect
}

func Frac(x float64) float64 {
	return x - float64(int(x))
}

func Lerp(a, b, t float64) float64 {
	return a + (b-a)*t
}

func GeneratePolygonPoints(sides int, radius float64, center Vec2) []Vec2 {
	points := make([]Vec2, sides)
	initAngle := -math.Pi / 2

	for i := range sides {
		angle := initAngle + (2.0*math.Pi*float64(i))/float64(sides)

		x := center.X + (radius * math.Cos(angle))
		y := center.Y + (radius * math.Sin(angle))

		x = math.Round(x*100) / 100
		y = math.Round(y*100) / 100

		points[i] = Vec2{X: x, Y: y}
	}

	return points
}
