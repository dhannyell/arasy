package wasm

import (
	"strconv"
	"testing"

	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/platform/wasm/tess"
	"github.com/dhannyell/frameforge/render"
)

func benchSpriteBatchInstances(n int, textureCount int) []render.SpriteInstance {
	instances := make([]render.SpriteInstance, n)
	for i := range n {
		x := float64(i%2000) * 20
		y := float64(i/2000) * 20
		texture := render.TextureID(1)
		if textureCount > 1 {
			texture = render.TextureID(i%textureCount + 1)
		}
		instances[i] = render.SpriteInstance{
			Texture: texture,
			Dest:    math2d.NewRect(math2d.Vec2{X: x, Y: y}, math2d.Vec2{X: 12, Y: 12}),
		}
	}
	return instances
}

func benchSpriteBatchTexSizes(textureCount int) tess.TexSizes {
	sizes := make(tess.TexSizes, textureCount+1)
	for i := 1; i <= textureCount; i++ {
		sizes[i] = [2]int{64, 64}
	}
	return sizes
}

// BenchmarkPlanSpriteBatchReplace measures the full retained sprite batch
// repack cost (record packing, range grouping, bounds) a replace pays. V1
// always fully repacks on replace — see planSpriteBatch in spritebatch.go.
func BenchmarkPlanSpriteBatchReplace(b *testing.B) {
	sizes := benchSpriteBatchTexSizes(4)

	for _, n := range []int{1000, 20000, 100000} {
		instances := benchSpriteBatchInstances(n, 1)
		b.Run("single_texture/"+strconv.Itoa(n), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				_ = planSpriteBatch(instances, sizes, false)
			}
		})
	}

	for _, n := range []int{1000, 20000, 100000} {
		instances := benchSpriteBatchInstances(n, 4)
		b.Run("four_textures_array/"+strconv.Itoa(n), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				_ = planSpriteBatch(instances, sizes, true)
			}
		})
	}
}
