package tilemap

import (
	"github.com/dhannyell/arasy/internal/renderbatch"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
)

// VisibleChunks returns the chunks whose bounds can reach the camera view.
// It applies Camera.View to all chunk corners before culling, as retained
// batches do, so an isometric view does not discard reachable tiles.
func VisibleChunks(l *Layer, cam render.Camera) []ChunkCoord {
	if l == nil || l.W <= 0 || l.H <= 0 || len(l.Tiles) < l.W*l.H {
		return nil
	}
	view := cam.Resolve().World
	if view.Width() <= 0 || view.Height() <= 0 {
		return nil
	}

	chunksX := (l.W-1)/ChunkSize + 1
	chunksY := (l.H-1)/ChunkSize + 1
	visible := make([]ChunkCoord, 0, chunksX*chunksY)
	for cy := range chunksY {
		for cx := range chunksX {
			bounds := renderbatch.BoundsForView(chunkBounds(l, cx, cy), cam.View)
			if renderbatch.IntersectsView(bounds, view) {
				visible = append(visible, ChunkCoord{X: cx, Y: cy})
			}
		}
	}
	return visible
}

func chunkBounds(l *Layer, cx, cy int) math2d.Rect {
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
	return math2d.NewRectFromCorners(first, last)
}
