package tilemap

import (
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
)

// ChunkSize is the width and height of one tilemap chunk in cells.
const ChunkSize = 32

// Atlas describes the texture layout used by a tile layer.
type Atlas struct {
	Texture  render.TextureID // Texture that contains the tiles.
	TileSize math2d.Vec2      // Size of one tile in the atlas.
	Columns  int              // Number of tiles in each atlas row.
	DrawSize math2d.Vec2      // World size of each tile; zero uses Layer.CellSize.
	Offset   math2d.Vec2      // World offset after bottom-aligning DrawSize.
	Spacing  math2d.Vec2      // Pixel gap between atlas tiles.
	Margin   math2d.Vec2      // Pixel border before the first atlas tile.
}

// ChunkCoord identifies a tilemap chunk by its column and row.
type ChunkCoord struct {
	X int
	Y int
}

// Layer stores tile indices and their world placement.
type Layer struct {
	Tiles     []int16             // Atlas indices in row-major order; -1 is empty.
	Flips     []render.SpriteFlip // Optional per-cell source flips; nil means no flips.
	Rotations []float64           // Optional per-cell rotations after flipping; nil means no rotation.
	W, H      int                 // Layer width and height in cells.
	Origin    math2d.Vec2         // World edge of tile (0,0).
	CellSize  math2d.Vec2         // Tile size in world units.
	dirty     map[ChunkCoord]struct{}
}

// Chunk returns the sprite instances for one chunk of the layer.
// The returned slice reuses buf when it has enough capacity.
func (l *Layer) Chunk(cx, cy int, a Atlas, buf []render.SpriteInstance) []render.SpriteInstance {
	if cx < 0 || cy < 0 || l.W <= 0 || l.H <= 0 || a.Columns <= 0 ||
		a.TileSize.X <= 0 || a.TileSize.Y <= 0 || len(l.Tiles) < l.W*l.H {
		return buf[:0]
	}

	minX := cx * ChunkSize
	minY := cy * ChunkSize
	if minX >= l.W || minY >= l.H {
		return buf[:0]
	}
	maxX := min(minX+ChunkSize, l.W)
	maxY := min(minY+ChunkSize, l.H)
	drawSize := a.DrawSize
	if drawSize.X <= 0 || drawSize.Y <= 0 {
		drawSize = l.CellSize
	}

	buf = buf[:0]
	for y := minY; y < maxY; y++ {
		for x := minX; x < maxX; x++ {
			index := y*l.W + x
			tile := l.Tiles[index]
			if tile < 0 {
				continue
			}
			atlasX := int(tile) % a.Columns
			atlasY := int(tile) / a.Columns
			position := l.Origin.Add(math2d.Vec2{
				X: float64(x)*l.CellSize.X + a.Offset.X,
				Y: float64(y)*l.CellSize.Y + l.CellSize.Y - drawSize.Y + a.Offset.Y,
			})
			instance := render.SpriteInstance{
				Texture:        a.Texture,
				ProjectCorners: true,
				Dest:           math2d.NewRect(position, drawSize),
				Src: math2d.NewRect(
					math2d.Vec2{
						X: a.Margin.X + float64(atlasX)*(a.TileSize.X+a.Spacing.X),
						Y: a.Margin.Y + float64(atlasY)*(a.TileSize.Y+a.Spacing.Y),
					},
					a.TileSize,
				),
			}
			if len(l.Flips) == len(l.Tiles) {
				instance.Flip = l.Flips[index]
			}
			if len(l.Rotations) == len(l.Tiles) {
				instance.Rotation = l.Rotations[index]
			}
			buf = append(buf, instance)
		}
	}
	return buf
}

// SetTile changes one cell and marks its chunk for regeneration.
// Coordinates outside the layer are ignored.
func (l *Layer) SetTile(x, y int, tile int16) {
	l.setTile(x, y, tile, render.FlipNone, false)
}

// SetTileFlip changes one cell's atlas index and source flip, allocating the
// optional flip slice only when a layer first needs it.
// Coordinates outside the layer are ignored.
func (l *Layer) SetTileFlip(x, y int, tile int16, flip render.SpriteFlip) {
	l.setTile(x, y, tile, flip, true)
}

func (l *Layer) setTile(x, y int, tile int16, flip render.SpriteFlip, setFlip bool) {
	if x < 0 || y < 0 || x >= l.W || y >= l.H || len(l.Tiles) < l.W*l.H {
		return
	}
	index := y*l.W + x
	if setFlip && len(l.Flips) != len(l.Tiles) {
		flips := make([]render.SpriteFlip, len(l.Tiles))
		copy(flips, l.Flips)
		l.Flips = flips
	}
	if l.Tiles[index] == tile && (!setFlip || l.Flips[index] == flip) {
		return
	}
	l.Tiles[index] = tile
	if setFlip {
		l.Flips[index] = flip
	}
	if l.dirty == nil {
		l.dirty = make(map[ChunkCoord]struct{})
	}
	l.dirty[ChunkCoord{X: x / ChunkSize, Y: y / ChunkSize}] = struct{}{}
}

// DirtyChunks returns and clears the chunks changed since the last call.
// The result is ordered by row and then by column.
func (l *Layer) DirtyChunks() []ChunkCoord {
	if len(l.dirty) == 0 {
		return nil
	}
	chunks := make([]ChunkCoord, 0, len(l.dirty))
	for chunk := range l.dirty {
		chunks = append(chunks, chunk)
	}
	for i := 1; i < len(chunks); i++ {
		for j := i; j > 0 && chunkBefore(chunks[j], chunks[j-1]); j-- {
			chunks[j], chunks[j-1] = chunks[j-1], chunks[j]
		}
	}
	l.dirty = nil
	return chunks
}

func chunkBefore(a, b ChunkCoord) bool {
	return a.Y < b.Y || a.Y == b.Y && a.X < b.X
}
