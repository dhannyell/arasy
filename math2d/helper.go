package math2d

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
