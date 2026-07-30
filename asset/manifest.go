package asset

import (
	"io/fs"
	"sort"

	"github.com/dhannyell/arasy/audio"
	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/particle"
	"github.com/dhannyell/arasy/render"
)

// ReservedNamePrefix identifies assets owned and injected by engine modules.
// Load rejects this prefix on every manifest-declared asset identifier so
// game content can never shadow an internal asset.
const ReservedNamePrefix = "__ar_"

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
	textures     []TextureDecl // index i holds ID i+1
	textByName   map[string]render.TextureID
	sprites      map[string]Sprite
	audio        []audio.Clip
	fonts        []FontDecl
	fontByName   map[string]render.FontID
	effects      []particle.EffectDecl
	effectByName map[string]render.ParticleEffectID
	splash       []SplashDecl

	// fsLayers are the FS layers Load was called with, retained so hosts can
	// read audio bytes (and any other path-addressed asset) from the same
	// source the manifest itself was resolved from, without a second
	// out-of-band FS. See FS().
	fsLayers []fs.FS
}

// Texture resolves a texture name to its handle. Unknown names return 0,
// which every backend renders as the magenta "missing texture" placeholder.
func (m Manifest) Texture(name string) render.TextureID {
	if m.textByName == nil {
		return 0
	}
	return m.textByName[name]
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

// Effect resolves a particle effect name to its handle. Unknown names
// return 0, which backends treat as "no effect" (no particles drawn).
func (m Manifest) Effect(name string) render.ParticleEffectID {
	if m.effectByName == nil {
		return 0
	}
	return m.effectByName[name]
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

// Effects returns the declared particle effects ordered by ID (1..N). The
// returned slice is shared - callers must not mutate it.
func (m Manifest) Effects() []particle.EffectDecl {
	return m.effects
}

// EffectDecl returns the declaration for a particle effect ID, or false if
// unknown.
func (m Manifest) EffectDecl(id render.ParticleEffectID) (particle.EffectDecl, bool) {
	if id == 0 || int(id) > len(m.effects) {
		return particle.EffectDecl{}, false
	}
	return m.effects[id-1], true
}

// WithEffect returns a copy of m with decl appended as a new particle effect
// and its resolved ID. Unlike Load (which assigns every ID by sorting all
// declared names together), WithEffect only ever appends at the end: it
// never reassigns or reorders the IDs of effects already in m, so it is
// safe to call after Load to register an effect the manifest file never
// declared (e.g. the engine's own boot splash spark, which has no
// manifest). If name is already registered, WithEffect returns m unchanged
// and the existing ID, making repeated internal asset injection idempotent.
func (m Manifest) WithEffect(name string, decl particle.EffectDecl) (Manifest, render.ParticleEffectID) {
	if id, ok := m.effectByName[name]; ok {
		return m, id
	}

	id := render.ParticleEffectID(len(m.effects) + 1)
	decl.Name = name

	effects := make([]particle.EffectDecl, len(m.effects), len(m.effects)+1)
	copy(effects, m.effects)
	effects = append(effects, decl)

	effectByName := make(map[string]render.ParticleEffectID, len(m.effectByName)+1)
	for k, v := range m.effectByName {
		effectByName[k] = v
	}
	effectByName[name] = id

	m.effects = effects
	m.effectByName = effectByName
	return m, id
}

// WithTexture returns a copy of m with a new static texture appended (its
// data is PNG/JPEG bytes, exactly like a manifest-declared texture) and its
// resolved ID. Like WithEffect, this only ever appends: it never reassigns
// or reorders the IDs of textures already in m, so it is safe to call after
// Load to register a texture the manifest file never declared (e.g. the
// engine's own boot splash particle sprites, which have no manifest entry).
// If name is already registered, WithTexture returns m unchanged and the
// existing ID, making repeated internal asset injection idempotent.
func (m Manifest) WithTexture(name string, data []byte) (Manifest, render.TextureID) {
	if id, ok := m.textByName[name]; ok {
		return m, id
	}

	id := render.TextureID(len(m.textures) + 1)

	textures := make([]TextureDecl, len(m.textures), len(m.textures)+1)
	copy(textures, m.textures)
	textures = append(textures, TextureDecl{ID: id, Name: name, Data: data})

	textByName := make(map[string]render.TextureID, len(m.textByName)+1)
	for k, v := range m.textByName {
		textByName[k] = v
	}
	textByName[name] = id

	m.textures = textures
	m.textByName = textByName
	return m, id
}

// AudioClips returns the declared audio clips ordered by ID. The returned
// slice is shared - callers must not mutate it.
func (m Manifest) AudioClips() []audio.Clip {
	return m.audio
}

// SplashDecl is one manifest-declared boot splash segment: a texture
// flip-book, resolved by name against the manifest's declared textures. It
// carries the same fields as the runtime splash segment's flip-book form;
// the runtime resolves Texture to a render.TextureID when building the boot
// playlist.
type SplashDecl struct {
	Texture      string
	CellW, CellH float64
	Columns      int
	Frames       uint16
	Rate         float32
	MinTime      float64
	Fade         float64
	Skippable    bool

	// Effect, when non-empty, names a particle effect declared in the
	// manifest's "effects" section to draw alongside this segment, emitted
	// from a fixed point expressed as a fraction of the viewport
	// (EffectAnchorX/Y, 0..1; default 0.5,0.5 — center). EffectIntensity
	// scales the effect's declared emit rate (0 defaults to 1).
	//
	// EffectAnchorX/Y is the only fraction-of-viewport field here — the named
	// effect's own Size/Speed/EmitExtent are "world units" (see
	// particle.EffectDecl), which resolve to screen pixels directly under
	// the splash's 1:1 camera. Don't confuse the two.
	Effect                       string
	EffectAnchorX, EffectAnchorY float64
	EffectIntensity              float64
}

// Splash returns the manifest-declared boot splash segments, in manifest
// order. The returned slice is shared - callers must not mutate it.
func (m Manifest) Splash() []SplashDecl {
	return m.splash
}

// FS returns a merged, read-only view of the FS layers this Manifest was
// loaded from, honoring the same last-layer-wins semantics as Load: a path
// present in more than one layer resolves to the last layer that has it.
// Hosts use this to read audio.Clip.Path bytes without needing a separate
// FS wired in out-of-band. The zero-value Manifest returns a safe, always
// fs.ErrNotExist FS rather than panicking.
func (m Manifest) FS() fs.FS {
	return layeredFS{layers: m.fsLayers}
}

func newManifest(
	textures map[string]TextureDecl,
	sprites map[string]Sprite,
	clips map[audio.ClipID]audio.Clip,
	fonts map[string]FontDecl,
	effects map[string]particle.EffectDecl,
) Manifest {
	textByName := make(map[string]render.TextureID, len(textures))
	textDecls := make([]TextureDecl, 0, len(textures))
	textNames := make([]string, 0, len(textures))

	fontByName := make(map[string]render.FontID, len(fonts))
	fontDecls := make([]FontDecl, 0, len(fonts))
	fontNames := make([]string, 0, len(fonts))

	audioIDs := make([]string, 0, len(clips))
	audioClips := make([]audio.Clip, 0, len(clips))

	effectByName := make(map[string]render.ParticleEffectID, len(effects))
	effectDecls := make([]particle.EffectDecl, 0, len(effects))
	effectNames := make([]string, 0, len(effects))

	for texture := range textures {
		textNames = append(textNames, texture)
	}
	sort.Strings(textNames)
	for i, name := range textNames {
		decl := textures[name]
		decl.ID = render.TextureID(i + 1)
		decl.Name = name
		textDecls = append(textDecls, decl)
		textByName[name] = decl.ID
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
		fontByName[name] = decl.ID
	}

	for id := range clips {
		audioIDs = append(audioIDs, string(id))
	}
	sort.Strings(audioIDs)
	for _, id := range audioIDs {
		audioClips = append(audioClips, clips[audio.ClipID(id)])
	}

	for effectName := range effects {
		effectNames = append(effectNames, effectName)
	}
	sort.Strings(effectNames)
	for i, name := range effectNames {
		decl := effects[name]
		decl.Name = name
		id := render.ParticleEffectID(i + 1)
		effectDecls = append(effectDecls, decl)
		effectByName[name] = id
	}

	return Manifest{
		textures:     textDecls,
		textByName:   textByName,
		sprites:      sprites,
		audio:        audioClips,
		fonts:        fontDecls,
		fontByName:   fontByName,
		effects:      effectDecls,
		effectByName: effectByName,
	}
}
