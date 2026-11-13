// Package slugtext places DrawText commands as Slug glyphs. It has no GPU types,
// so the raster renderer and the GPU hosts share one layout.
package slugtext

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"

	slug "github.com/dhannyell/go-slug"
	"github.com/dhannyell/go-slug/text"

	"github.com/dhannyell/arasy/asset/typeface"
	"github.com/dhannyell/arasy/internal/render/slugcore"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
)

// DefaultMaxBytes limits the Pack of an Engine. It is half of the storage buffer
// limit that WebGPU guarantees.
const DefaultMaxBytes = 64 << 20

// shapeEntries limits the shape cache of an Engine. One generation holds half of it,
// so a frame with more unique labels than that shapes them again.
const shapeEntries = 8192

var _ slugcore.Engine = (*Engine)(nil)

// Engine keeps the Pack and the shapers of one renderer. It is not safe for
// concurrent use.
type Engine struct {
	lookup func(render.FontID) *typeface.Face
	fonts  map[render.FontID]shaper
	pack   slug.Pack
	shapes *text.ShapeCache
	reset  bool // EndFrame must reset the Pack

	instanceGlyphs []slugcore.Glyph
	err            error
}

// shaper is the go-slug font of one Face. font is nil if the Face did not parse.
type shaper struct {
	face *typeface.Face
	font *text.Font
}

// New returns an Engine. lookup finds the font of an ID other than 0; ID 0 is the
// default font of the engine.
func New(lookup func(render.FontID) *typeface.Face) *Engine {
	e := &Engine{lookup: lookup, fonts: map[render.FontID]shaper{}, shapes: text.NewShapeCache(shapeEntries)}
	e.pack.MaxBytes = DefaultMaxBytes
	return e
}

// Pack returns the Pack that holds the entries of the glyphs of Append.
func (e *Engine) Pack() *slug.Pack { return &e.pack }

// EndFrame ends a frame. If the Pack became full, EndFrame resets it, and the next
// frame adds again only the glyphs that it uses. A reset never happens in a frame,
// because the glyphs of that frame still refer to the Pack.
func (e *Engine) EndFrame() {
	if e.reset {
		e.pack.Reset()
		e.reset = false
	}
}

// Append appends the glyphs of cmd to dst. A full Pack returns the glyphs that fit
// and an error that wraps slug.ErrPackFull.
func (e *Engine) Append(dst []slugcore.Glyph, cmd render.DrawText, cam render.Camera) ([]slugcore.Glyph, error) {
	if cmd.Text == "" || cmd.Color.A <= 0 {
		return dst, nil
	}
	if cmd.Size <= 0 {
		return dst, fmt.Errorf("%w: size %v", slugcore.ErrNoFont, cmd.Size)
	}
	f := e.font(cmd.Font)
	if f == nil {
		return dst, fmt.Errorf("%w: id %d", slugcore.ErrNoFont, cmd.Font)
	}

	size := float32(cam.WorldScalarToScreen(cmd.Size))
	spacing := float32(cam.WorldScalarToScreen(cmd.LetterSpacing))
	color := [4]float32{float32(cmd.Color.R), float32(cmd.Color.G), float32(cmd.Color.B), float32(cmd.Color.A)}
	lines := e.shapes.Shape(f, cmd.Text, text.Options{})
	toScreen := frame(cmd, cam)

	baseline := firstBaseline(cmd.Baseline, f.Metrics(), len(lines), size)
	for _, line := range lines {
		x := startX(cmd.Align, line, size, spacing)
		for i, g := range line.Glyphs {
			if i > 0 && g.Cluster != line.Glyphs[i-1].Cluster {
				x += spacing
			}
			entry, err := f.Add(&e.pack, g)
			if errors.Is(err, slug.ErrPackFull) {
				e.reset = true
			}
			if err != nil {
				return dst, err
			}
			view := g.At(size).Then(slug.Affine{1, 0, 0, 1, x, baseline}).Then(toScreen)
			dst = append(dst, slugcore.Glyph{Entry: entry, View: view, Color: color})
		}
		baseline += line.Height() * size
	}
	return dst, nil
}

// Has reports if the Engine can draw text in the font.
func (e *Engine) Has(id render.FontID) bool { return e.font(id) != nil }

// AppendInstances appends one record of tess.SlugInstanceStride bytes for each layer of each
// glyph of cmd, and returns the screen bounds of these glyphs. A failed command appends the
// glyphs that fit, and TakeErr reports it. clipMin and clipMax are in screen pixels.
func (e *Engine) AppendInstances(dst []byte, cmd render.DrawText, cam render.Camera, clipMin, clipMax math2d.Vec2) ([]byte, math2d.Rect) {
	var err error
	e.instanceGlyphs, err = e.Append(e.instanceGlyphs[:0], cmd, cam)
	if err != nil && e.err == nil {
		e.err = err
	}
	clip := [4]float32{float32(clipMin.X), float32(clipMin.Y), float32(clipMax.X), float32(clipMax.Y)}
	var bounds math2d.Rect
	has := false
	for _, g := range e.instanceGlyphs {
		if g.Entry.LayerCount == 0 {
			continue
		}
		expandBounds(&bounds, &has, g)
		for layer := range g.Entry.LayerCount {
			dst = appendInstance(dst, g, clip, g.Entry.FirstLayer+layer)
		}
	}
	return dst, bounds
}

// expandBounds adds the screen corners of g, with the margin of the shader dilation.
func expandBounds(bounds *math2d.Rect, has *bool, g slugcore.Glyph) {
	const margin = 2 // pixels
	v, b := g.View, g.Entry.Bounds
	for _, c := range [...][2]float32{{b.Min.X, b.Min.Y}, {b.Max.X, b.Min.Y}, {b.Max.X, b.Max.Y}, {b.Min.X, b.Max.Y}} {
		x := float64(v[0]*c[0] + v[2]*c[1] + v[4])
		y := float64(v[1]*c[0] + v[3]*c[1] + v[5])
		if !*has {
			*bounds = math2d.Rect{Min: math2d.Vec2{X: x - margin, Y: y - margin}, Max: math2d.Vec2{X: x + margin, Y: y + margin}}
			*has = true
			continue
		}
		bounds.Min.X, bounds.Min.Y = min(bounds.Min.X, x-margin), min(bounds.Min.Y, y-margin)
		bounds.Max.X, bounds.Max.Y = max(bounds.Max.X, x+margin), max(bounds.Max.Y, y+margin)
	}
}

// TakeErr returns the first error of AppendInstances since the last call.
func (e *Engine) TakeErr() error {
	err := e.err
	e.err = nil
	return err
}

// appendInstance writes the Instance struct of slug_text.wgsl.
func appendInstance(dst []byte, g slugcore.Glyph, clip [4]float32, layer uint32) []byte {
	v := g.View
	for _, f := range [...]float32{
		v[0], v[1], v[2], v[3],
		g.Color[0], g.Color[1], g.Color[2], g.Color[3],
		v[4], v[5],
		clip[0], clip[1], clip[2], clip[3],
	} {
		dst = binary.LittleEndian.AppendUint32(dst, math.Float32bits(f))
	}
	dst = binary.LittleEndian.AppendUint32(dst, layer)
	return binary.LittleEndian.AppendUint32(dst, 0)
}

func (e *Engine) font(id render.FontID) *text.Font {
	var face *typeface.Face
	if id == 0 {
		face = typeface.DefaultFace()
	} else if e.lookup != nil {
		face = e.lookup(id)
	}
	if face == nil {
		return nil
	}
	// A new Face under one ID is a font reload, so the next frame drops old glyphs.
	cached, ok := e.fonts[id]
	if ok && cached.face == face {
		return cached.font
	}
	if ok {
		e.reset = true
	}
	f, err := text.Parse(face.Data(), uint32(id))
	if err != nil {
		f = nil
	}
	e.fonts[id] = shaper{face: face, font: f}
	return f
}

// startX returns the X of the first glyph of line, from the anchor. The Engine
// computes it because LetterSpacing changes the width that text.Options.Align reads.
func startX(align render.TextAlign, line text.Line, size, spacing float32) float32 {
	gaps := 0
	for i := 1; i < len(line.Glyphs); i++ {
		if line.Glyphs[i].Cluster != line.Glyphs[i-1].Cluster {
			gaps++
		}
	}
	width := line.Width*size + float32(gaps)*spacing
	switch align {
	case render.AlignCenter:
		return -width / 2
	case render.AlignRight:
		return -width
	default:
		return 0
	}
}

// firstBaseline returns the Y of the first baseline, from the anchor, Y down.
func firstBaseline(b render.TextBaseline, m text.Metrics, lines int, size float32) float32 {
	ascent := m.Ascent * size
	height := (m.Ascent-m.Descent)*size + float32(max(lines-1, 0))*m.Height()*size
	switch b {
	case render.BaselineTop:
		return ascent
	case render.BaselineMiddle:
		return ascent - height/2
	case render.BaselineBottom:
		return ascent - height
	default:
		return 0
	}
}

// frame maps pixels from the anchor to the screen. It follows the raster order:
// Rotation in screen space, then Transform in world space.
func frame(cmd render.DrawText, cam render.Camera) slug.Affine {
	anchor := cam.WorldToScreen(cmd.Position)
	sin, cos := math.Sincos(cmd.Rotation)
	apply := func(x, y float64) math2d.Vec2 {
		p := math2d.Vec2{X: anchor.X + x*cos - y*sin, Y: anchor.Y + x*sin + y*cos}
		if cmd.Transform.IsIdentity() {
			return p
		}
		return cam.WorldToScreen(cmd.Transform.Apply(cam.ScreenToWorld(p)))
	}
	o, ex, ey := apply(0, 0), apply(1, 0), apply(0, 1)
	return slug.Affine{
		float32(ex.X - o.X), float32(ex.Y - o.Y),
		float32(ey.X - o.X), float32(ey.Y - o.Y),
		float32(o.X), float32(o.Y),
	}
}
