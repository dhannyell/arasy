package outpost

import (
	"image"
	"image/color"
)

const (
	terrainAtlasCell    = 32
	terrainAtlasColumns = 4
)

// terrainAtlasImage creates the small atlas referenced by the generated Tiled
// ground map. Its pixels are presentation-only.
func terrainAtlasImage() *image.RGBA {
	colors := [...]color.RGBA{
		{R: 126, G: 156, B: 90, A: 255},
		{R: 103, G: 133, B: 75, A: 255},
		{R: 122, G: 113, B: 104, A: 255},
		{R: 148, G: 116, B: 86, A: 255},
	}
	img := image.NewRGBA(image.Rect(0, 0, terrainAtlasColumns*terrainAtlasCell, terrainAtlasCell))
	for tile, base := range colors {
		offsetX := tile * terrainAtlasCell
		for y := range terrainAtlasCell {
			for x := range terrainAtlasCell {
				pixel := base
				if x == 0 || y == 0 || x == terrainAtlasCell-1 || y == terrainAtlasCell-1 {
					pixel = color.RGBA{R: base.R * 9 / 10, G: base.G * 9 / 10, B: base.B * 9 / 10, A: 255}
				} else if (x+y+tile*3)%11 == 0 {
					pixel = color.RGBA{R: minByte(int(base.R) + 4), G: minByte(int(base.G) + 4), B: minByte(int(base.B) + 4), A: 255}
				}
				img.SetRGBA(offsetX+x, y, pixel)
			}
		}
	}
	return img
}

func minByte(value int) uint8 {
	if value > 255 {
		return 255
	}
	return uint8(value)
}
