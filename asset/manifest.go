package asset

import (
	"sort"

	"github.com/dhannyell/frameforge/audio"
	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/render"
)

// Sprite is a named sub-rectangle of a texture (atlas/sprite-sheet entry).
type Sprite struct {
	Texture render.TextureID
	// Src is the sprite rectangle in texture pixel coordinates.
	Src math2d.Rect
}

// TextureDecl is one declared texture with its encoded bytes (PNG/JPEG),
// exactly as read from the winning FS layer. Platforms build their
// TextureID-indexed registries from Manifest.Textures().
type TextureDecl struct {
	ID   render.TextureID
	Name string
	Data []byte

	// Dynamic is true when the texture was declared via the "dynamic" section
	// of the manifest. Dynamic textures have no file-based pixel data; their
	// content arrives at runtime via RenderContext.UpdateTexture. W and H
	// specify the fixed dimensions of the texture; they are 0 for static
	// textures.
	Dynamic bool
	W, H    int
}

// FontDecl is one declared font with its raw TTF/OTF bytes.
type FontDecl struct {
	ID   render.FontID
	Name string
	Data []byte
}

// Manifest is the resolved, immutable asset declaration set. The zero value
// is a valid empty manifest (games without assets never touch it).
type Manifest struct {
	textures   []TextureDecl // index i holds ID i+1
	byName     map[string]render.TextureID
	sprites    map[string]Sprite
	audio      []audio.Clip
	fonts      []FontDecl
	fontByName map[string]render.FontID
}

// Texture resolves a texture name to its handle. Unknown names return 0,
// which every backend renders as the magenta "missing texture" placeholder.
func (m Manifest) Texture(name string) render.TextureID {
	if m.byName == nil {
		return 0
	}
	return m.byName[name]
}

// Sprite resolves a sprite name. Unknown names return the zero Sprite
// (Texture 0), which draws as the missing-texture placeholder.
func (m Manifest) Sprite(name string) Sprite {
	if m.sprites == nil {
		return Sprite{}
	}
	return m.sprites[name]
}

func (m Manifest) Font(name string) render.FontID {
	if m.fontByName == nil {
		return 0
	}
	return m.fontByName[name]
}

// Textures returns the declarations ordered by ID (1..N). The returned slice
// is shared - callers must not mutate it.
func (m Manifest) Textures() []TextureDecl {
	return m.textures
}

func (m Manifest) Fonts() []FontDecl {
	return m.fonts
}

// TextureDecl returns the declaration for a texture ID, or false if unknown.
func (m Manifest) TextureDecl(id render.TextureID) (TextureDecl, bool) {
	if id == 0 || int(id) > len(m.textures) {
		return TextureDecl{}, false
	}
	return m.textures[id-1], true
}

// IsDynamic reports whether the given texture ID is a dynamic texture.
func (m Manifest) IsDynamic(id render.TextureID) bool {
	d, ok := m.TextureDecl(id)
	return ok && d.Dynamic
}

// AudioClips returns the declared audio clips ordered by ID. The returned
// slice is shared - callers must not mutate it.
func (m Manifest) AudioClips() []audio.Clip {
	return m.audio
}

func newManifest(textures map[string]TextureDecl, sprites map[string]Sprite, clips map[audio.ClipID]audio.Clip, fonts map[string]FontDecl) Manifest {
	byName := make(map[string]render.TextureID, len(textures))
	decls := make([]TextureDecl, 0, len(textures))
	names := make([]string, 0, len(textures))
	byFontName := make(map[string]render.FontID, len(fonts))
	fontDecls := make([]FontDecl, 0, len(fonts))
	fontNames := make([]string, 0, len(fonts))
	for name := range textures {
		names = append(names, name)
	}
	sort.Strings(names)
	for i, name := range names {
		decl := textures[name]
		decl.ID = render.TextureID(i + 1)
		decl.Name = name
		decls = append(decls, decl)
		byName[name] = decl.ID
	}

	for fontName := range fonts {
		fontNames = append(fontNames, fontName)
	}
	sort.Strings(fontNames)
	for i, name := range fontNames {
		decl := fonts[name]
		decl.ID = render.FontID(i + 1)
		decl.Name = name
		fontDecls = append(fontDecls, decl)
		byFontName[name] = decl.ID
	}

	audioIDs := make([]string, 0, len(clips))
	for id := range clips {
		audioIDs = append(audioIDs, string(id))
	}
	sort.Strings(audioIDs)
	audioClips := make([]audio.Clip, 0, len(audioIDs))
	for _, id := range audioIDs {
		audioClips = append(audioClips, clips[audio.ClipID(id)])
	}

	return Manifest{
		textures:   decls,
		byName:     byName,
		sprites:    sprites,
		audio:      audioClips,
		fonts:      fontDecls,
		fontByName: byFontName,
	}
}
