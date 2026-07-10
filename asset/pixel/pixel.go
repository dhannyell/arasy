// Package pixel decodes declared textures into premultiplied-alpha RGBA
// images and builds the TextureID-indexed registries that CPU/GPU backends
// consume. It is kept out of the core asset package so that runtime (which
// carries the Manifest) never links image decoders.
package pixel

import (
	"bytes"
	"fmt"
	"image"
	"image/draw"
	_ "image/jpeg"
	_ "image/png"

	"github.com/dhannyell/frameforge/asset"
	"github.com/dhannyell/frameforge/render"
)

// DecodeRGBA decodes PNG or JPEG bytes into a premultiplied-alpha RGBA
// image anchored at the origin.
func DecodeRGBA(data []byte) (*image.RGBA, error) {
	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	out := image.NewRGBA(image.Rect(0, 0, src.Bounds().Dx(), src.Bounds().Dy()))
	draw.Draw(out, out.Bounds(), src, src.Bounds().Min, draw.Src)
	return out, nil
}

// Registry maps TextureID to decoded pixels. Index 0 is always nil
// (TextureID 0 means "missing texture").
type Registry []*image.RGBA

// BuildRegistry decodes every texture in the manifest.
func BuildRegistry(m asset.Manifest) (Registry, error) {
	decls := m.Textures()
	reg := make(Registry, len(decls)+1)
	for _, d := range decls {
		if d.Dynamic {
			reg[d.ID] = image.NewRGBA(image.Rect(0, 0, d.W, d.H))
			continue
		}
		img, err := DecodeRGBA(d.Data)
		if err != nil {
			return nil, fmt.Errorf("decode texture %q: %w", d.Name, err)
		}
		reg[d.ID] = img
	}
	return reg, nil
}

// Lookup returns the pixels for id, or nil when the id is unknown.
func (r Registry) Lookup(id render.TextureID) *image.RGBA {
	if id == 0 || int(id) >= len(r) {
		return nil
	}
	return r[id]
}
