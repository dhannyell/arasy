// Package pixel decodes declared textures into premultiplied-alpha RGBA
// images and builds the TextureID-indexed registries that CPU/GPU backends
// consume. It is kept out of the core asset package so that runtime (which
// carries the Manifest) never links image decoders.
package pixel

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/draw"
	_ "image/jpeg"
	_ "image/png"

	"github.com/dhannyell/arasy/asset"
	"github.com/dhannyell/arasy/render"
)

const (
	maxImageDimension = 8192
	maxImagePixels    = 16 << 20
)

// DecodeRGBA decodes PNG or JPEG bytes into a premultiplied-alpha RGBA
// image anchored at the origin.
func DecodeRGBA(data []byte) (*image.RGBA, error) {
	config, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	if err := validateImageDimensions(config.Width, config.Height); err != nil {
		return nil, err
	}

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

// BuildRegistry decodes every texture in the manifest. A texture that fails
// to decode is left nil in the registry (the same "missing texture" state
// Lookup already reports for an unknown id) instead of aborting the whole
// registry, so one corrupt asset never blocks the others; every decode error
// is collected and returned joined.
func BuildRegistry(m asset.Manifest) (Registry, error) {
	decls := m.Textures()
	reg := make(Registry, len(decls)+1)
	var errs []error
	for _, d := range decls {
		if d.Dynamic {
			if err := validateImageDimensions(d.W, d.H); err != nil {
				errs = append(errs, fmt.Errorf("allocate dynamic texture %q: %w", d.Name, err))
				continue
			}
			reg[d.ID] = image.NewRGBA(image.Rect(0, 0, d.W, d.H))
			continue
		}
		img, err := DecodeRGBA(d.Data)
		if err != nil {
			errs = append(errs, fmt.Errorf("decode texture %q: %w", d.Name, err))
			continue
		}
		reg[d.ID] = img
	}
	return reg, errors.Join(errs...)
}

func validateImageDimensions(width, height int) error {
	if width <= 0 || height <= 0 {
		return fmt.Errorf("image dimensions %dx%d must be positive", width, height)
	}
	if width > maxImageDimension || height > maxImageDimension {
		return fmt.Errorf("image dimensions %dx%d exceed per-side limit %d", width, height, maxImageDimension)
	}
	if width > maxImagePixels/height {
		return fmt.Errorf("image dimensions %dx%d exceed pixel limit %d", width, height, maxImagePixels)
	}
	return nil
}

// Lookup returns the pixels for id, or nil when the id is unknown.
func (r Registry) Lookup(id render.TextureID) *image.RGBA {
	if id == 0 || int(id) >= len(r) {
		return nil
	}
	return r[id]
}
