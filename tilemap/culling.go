package tilemap

import (
	"github.com/dhannyell/arasy/internal/renderbatch"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
)

// VisibleChunks returns the chunks whose bounds can reach the camera view.
// When an atlas is supplied, its tall-tile extent and offset expand the bounds
// so culling does not discard an overlapping neighbor chunk. It applies
// Camera.View to all chunk corners before culling, as retained batches do.
func VisibleChunks(l *Layer, cam render.Camera, atlas ...Atlas) []ChunkCoord {
	return visibleChunksInto(nil, l, cam, atlas...)
}

// visibleChunksInto appends the visible chunks to dst so callers can reuse a
// buffer across frames.
func visibleChunksInto(dst []ChunkCoord, l *Layer, cam render.Camera, atlas ...Atlas) []ChunkCoord {
	if l == nil || l.W <= 0 || l.H <= 0 || len(l.Tiles) < l.W*l.H {
		return dst
	}
	view := cam.Resolve().World
	if view.Width() <= 0 || view.Height() <= 0 {
		return dst
	}

	chunksX := l.chunksWide()
	chunksY := l.chunksHigh()
	for cy := range chunksY {
		for cx := range chunksX {
			bounds := renderbatch.BoundsForView(chunkBounds(l, cx, cy, atlas...), cam.View)
			if renderbatch.IntersectsView(bounds, view) {
				dst = append(dst, ChunkCoord{X: cx, Y: cy})
			}
		}
	}
	return dst
}

func chunkBounds(l *Layer, cx, cy int, atlas ...Atlas) math2d.Rect {
	minX := cx * ChunkSize
	minY := cy * ChunkSize
	maxX := min(minX+ChunkSize, l.W)
	maxY := min(minY+ChunkSize, l.H)
	first := l.Origin.Add(math2d.Vec2{
		X: float64(minX) * l.CellSize.X,
		Y: float64(minY) * l.CellSize.Y,
	})
	last := l.Origin.Add(math2d.Vec2{
		X: float64(maxX) * l.CellSize.X,
		Y: float64(maxY) * l.CellSize.Y,
	})
	if len(atlas) == 0 {
		return math2d.NewRectFromCorners(first, last)
	}
	drawSize := atlas[0].effectiveDrawSize(l.CellSize)
	offset := atlas[0].Offset
	min := first.Add(math2d.Vec2{X: offset.X, Y: l.CellSize.Y - drawSize.Y + offset.Y})
	max := last.Add(math2d.Vec2{X: offset.X + drawSize.X - l.CellSize.X, Y: offset.Y})
	return math2d.NewRectFromCorners(min, max)
}
