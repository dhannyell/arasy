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
}

// Manifest is the resolved, immutable asset declaration set. The zero value
// is a valid empty manifest (games without assets never touch it).
type Manifest struct {
	textures []TextureDecl // index i holds ID i+1
	byName   map[string]render.TextureID
	sprites  map[string]Sprite
	audio    []audio.Clip
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

// Textures returns the declarations ordered by ID (1..N). The returned slice
// is shared - callers must not mutate it.
func (m Manifest) Textures() []TextureDecl {
	return m.textures
}

// AudioClips returns the declared audio clips ordered by ID. The returned
// slice is shared - callers must not mutate it.
func (m Manifest) AudioClips() []audio.Clip {
	return m.audio
}

func newManifest(textures map[string]TextureDecl, sprites map[string]Sprite, clips map[audio.ClipID]audio.Clip) Manifest {
	byName := make(map[string]render.TextureID, len(textures))
	decls := make([]TextureDecl, 0, len(textures))
	names := make([]string, 0, len(textures))
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

	audioIDs := make([]string, 0, len(clips))
	for id := range clips {
		audioIDs = append(audioIDs, string(id))
	}
	sort.Strings(audioIDs)
	audioClips := make([]audio.Clip, 0, len(audioIDs))
	for _, id := range audioIDs {
		audioClips = append(audioClips, clips[audio.ClipID(id)])
	}

	return Manifest{textures: decls, byName: byName, sprites: sprites, audio: audioClips}
}
