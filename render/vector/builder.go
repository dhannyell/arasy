package vector

import "github.com/dhannyell/arasy/math2d"

// Builder records path verbs. Build returns an immutable copy.
type Builder struct {
	segments []segment
	current  math2d.Vec2
	start    math2d.Vec2
	open     bool
	err      error
}

func (b *Builder) MoveTo(point math2d.Vec2) {
	if !finitePoint(point) {
		b.err = ErrNonFinite
		return
	}
	b.segments = append(b.segments, segment{verb: verbMove, p0: point})
	b.current, b.start, b.open = point, point, true
}

func (b *Builder) LineTo(point math2d.Vec2) {
	if !b.open {
		b.err = ErrNoCurrentSubpath
		return
	}
	if !finitePoint(point) {
		b.err = ErrNonFinite
		return
	}
	b.segments = append(b.segments, segment{verb: verbLine, p0: point})
	b.current = point
}

func (b *Builder) QuadTo(control, to math2d.Vec2) {
	if !b.open {
		b.err = ErrNoCurrentSubpath
		return
	}
	if !finitePoint(control) || !finitePoint(to) {
		b.err = ErrNonFinite
		return
	}
	b.segments = append(b.segments, segment{verb: verbQuad, p0: control, p1: to})
	b.current = to
}

func (b *Builder) CubicTo(control1, control2, to math2d.Vec2) {
	if !b.open {
		b.err = ErrNoCurrentSubpath
		return
	}
	if !finitePoint(control1) || !finitePoint(control2) || !finitePoint(to) {
		b.err = ErrNonFinite
		return
	}
	b.segments = append(b.segments, segment{verb: verbCubic, p0: control1, p1: control2, p2: to})
	b.current = to
}

func (b *Builder) Close() {
	if !b.open {
		b.err = ErrNoCurrentSubpath
		return
	}
	b.segments = append(b.segments, segment{verb: verbClose})
	b.current, b.open = b.start, false
}

func (b *Builder) Build() (Path, error) {
	if b.err != nil {
		return Path{}, b.err
	}
	if len(b.segments) == 0 {
		return Path{}, ErrEmptyPath
	}
	segments := append([]segment(nil), b.segments...)
	return Path{segments: segments, cache: &flattenCache{}}, nil
}
