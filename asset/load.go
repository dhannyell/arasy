package asset

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"

	"github.com/dhannyell/frameforge/audio"
	"github.com/dhannyell/frameforge/math2d"
)

const (
	maxEncodedTextureBytes int64 = 16 << 20
	maxFontBytes           int64 = 16 << 20
	maxAudioClipBytes      int64 = 16 << 20
	maxTotalAudioBytes     int64 = 64 << 20
)

type manifestFile struct {
	Version  int            `json:"version"`
	Textures []textureEntry `json:"textures"`
	Dynamic  []dynamicEntry `json:"dynamic"`
	Sprites  []spriteEntry  `json:"sprites"`
	Fonts    []fontEntry    `json:"fonts"`
	Audio    []audioEntry   `json:"audio"`
}

type textureEntry struct {
	Name string `json:"name"`
	File string `json:"file"`
	W    int    `json:"w"`
	H    int    `json:"h"`
}

type dynamicEntry struct {
	Name string `json:"name"`
	W    int    `json:"w"`
	H    int    `json:"h"`
}

type spriteEntry struct {
	Name    string  `json:"name"`
	Texture string  `json:"texture"`
	X       float64 `json:"x"`
	Y       float64 `json:"y"`
	W       float64 `json:"w"`
	H       float64 `json:"h"`
}

type audioEntry struct {
	ID   string `json:"id"`
	File string `json:"file"`
}

type fontEntry struct {
	Name string `json:"name"`
	File string `json:"file"`
}

// Load reads manifest.json from the given FS layers and resolves every
// declared asset. Later layers take priority ("last wins"), at two levels:
//   - manifest entries: a texture/sprite name redeclared in a later layer
//     replaces the earlier declaration;
//   - file bytes: the file path of a winning declaration is read from the
//     LAST layer that contains it, so a mod layer can replace a PNG
//     without shipping its own manifest.json.
//
// Fonts are optional manifest entries with a non-empty name and file path;
// their raw TTF/OTF bytes are loaded with the same last-layer-wins rule as
// textures. A layer without manifest.json is valid (file-override-only layer),
// but at least one layer must declare one.
func Load(layers ...fs.FS) (Manifest, error) {
	if len(layers) == 0 {
		return Manifest{}, errors.New("asset: Load needs at least one FS layer")
	}

	texByName := map[string]textureEntry{}
	sprByName := map[string]spriteEntry{}
	fntByName := map[string]fontEntry{}
	audioByID := map[string]audioEntry{}
	sawManifest := false

	for _, layer := range layers {
		data, err := fs.ReadFile(layer, "manifest.json")
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return Manifest{}, fmt.Errorf("asset: read manifest.json: %w", err)
		}
		var mf manifestFile
		if err := json.Unmarshal(data, &mf); err != nil {
			return Manifest{}, fmt.Errorf("asset: parse manifest.json: %w", err)
		}
		if mf.Version != 1 {
			return Manifest{}, fmt.Errorf("asset: unsupported manifest version %d", mf.Version)
		}
		if err := validateManifestEntries(mf.Textures); err != nil {
			return Manifest{}, err
		}
		if err := validateSpriteEntries(mf.Sprites); err != nil {
			return Manifest{}, err
		}
		if err := validateFontEntries(mf.Fonts); err != nil {
			return Manifest{}, err
		}
		if err := validateDynamicEntries(mf.Dynamic); err != nil {
			return Manifest{}, err
		}
		if err := validateAudioEntries(mf.Audio); err != nil {
			return Manifest{}, err
		}
		for _, entry := range mf.Textures {
			texByName[entry.Name] = entry
		}
		for _, entry := range mf.Dynamic {
			texByName[entry.Name] = textureEntry{Name: entry.Name, W: entry.W, H: entry.H}
		}
		for _, entry := range mf.Sprites {
			sprByName[entry.Name] = entry
		}
		for _, entry := range mf.Fonts {
			fntByName[entry.Name] = fontEntry{Name: entry.Name, File: entry.File}
		}
		for _, entry := range mf.Audio {
			audioByID[entry.ID] = entry
		}
		sawManifest = true
	}

	if !sawManifest {
		return Manifest{}, errors.New("asset: no manifest.json found in any layer")
	}

	// newManifest owns ID and Name assignment (sorted by name); here we only
	// read the winning bytes for each declared texture.
	textures := make(map[string]TextureDecl, len(texByName))
	sprites := make(map[string]Sprite, len(sprByName))
	fonts := make(map[string]FontDecl, len(fntByName))
	clips := make(map[audio.ClipID]audio.Clip, len(audioByID))
	for name, entry := range texByName {
		if entry.File == "" {
			textures[name] = TextureDecl{Dynamic: true, W: entry.W, H: entry.H}
			continue
		}
		data, err := readLayered(layers, entry.File, maxEncodedTextureBytes)
		if err != nil {
			return Manifest{}, fmt.Errorf("asset: texture %q: %w", name, err)
		}
		textures[name] = TextureDecl{Data: data}
	}
	for name, entry := range fntByName {
		data, err := readLayered(layers, entry.File, maxFontBytes)
		if err != nil {
			return Manifest{}, fmt.Errorf("asset: font %q: %w", name, err)
		}
		if len(data) == 0 {
			return Manifest{}, fmt.Errorf("asset: font %q: file %q is empty", name, entry.File)
		}
		fonts[name] = FontDecl{Data: data}
	}
	var totalAudioBytes int64
	for id, entry := range audioByID {
		// Audio bytes stay on disk: the host resolves and streams them (HTTP on
		// web, embed on wasm). Only confirm the winning layer has the file so a
		// bad path fails at load, not at first playback.
		info, err := statLayered(layers, entry.File)
		if err != nil {
			return Manifest{}, fmt.Errorf("asset: audio %q: %w", id, err)
		}
		if err := validateFileSize(entry.File, info.Size(), maxAudioClipBytes); err != nil {
			return Manifest{}, fmt.Errorf("asset: audio %q: %w", id, err)
		}
		if totalAudioBytes > maxTotalAudioBytes-info.Size() {
			return Manifest{}, fmt.Errorf("asset: total audio size exceeds limit %d bytes", maxTotalAudioBytes)
		}
		totalAudioBytes += info.Size()
		clips[audio.ClipID(id)] = audio.Clip{ID: audio.ClipID(id), Path: entry.File}
	}

	manifest := newManifest(textures, nil, clips, fonts)
	manifest.fsLayers = layers
	for name, s := range sprByName {
		texID, ok := manifest.byName[s.Texture]
		if !ok {
			return Manifest{}, fmt.Errorf("asset: sprite %q references unknown texture %q", name, s.Texture)
		}
		sprites[name] = Sprite{
			Texture: texID,
			Src: math2d.NewRect(
				math2d.Vec2{X: s.X, Y: s.Y},
				math2d.Vec2{X: s.W, Y: s.H},
			),
		}
	}
	manifest.sprites = sprites
	return manifest, nil
}

func validateManifestEntries(entries []textureEntry) error {
	seen := map[string]struct{}{}
	for _, entry := range entries {
		if _, ok := seen[entry.Name]; ok {
			return fmt.Errorf("asset: duplicate texture %q in manifest.json", entry.Name)
		}
		if entry.File != "" {
			if err := validateAssetPath("texture", entry.Name, entry.File); err != nil {
				return err
			}
		}
		seen[entry.Name] = struct{}{}
	}
	return nil
}

func validateSpriteEntries(entries []spriteEntry) error {
	seen := map[string]struct{}{}
	for _, entry := range entries {
		if _, ok := seen[entry.Name]; ok {
			return fmt.Errorf("asset: duplicate sprite %q in manifest.json", entry.Name)
		}
		seen[entry.Name] = struct{}{}
	}
	return nil
}

func validateFontEntries(entries []fontEntry) error {
	seen := map[string]struct{}{}
	for _, entry := range entries {
		if entry.Name == "" {
			return errors.New("asset: font name must not be empty in manifest.json")
		}
		if entry.File == "" {
			return fmt.Errorf("asset: font %q has an empty file path in manifest.json", entry.Name)
		}
		if err := validateAssetPath("font", entry.Name, entry.File); err != nil {
			return err
		}
		if _, ok := seen[entry.Name]; ok {
			return fmt.Errorf("asset: duplicate font %q in manifest.json", entry.Name)
		}
		seen[entry.Name] = struct{}{}
	}
	return nil
}

func validateAudioEntries(entries []audioEntry) error {
	seen := map[string]struct{}{}
	for _, entry := range entries {
		if entry.ID == "" {
			return errors.New("asset: audio id must not be empty in manifest.json")
		}
		if entry.File == "" {
			return fmt.Errorf("asset: audio %q has an empty file path in manifest.json", entry.ID)
		}
		if err := validateAssetPath("audio", entry.ID, entry.File); err != nil {
			return err
		}
		if _, ok := seen[entry.ID]; ok {
			return fmt.Errorf("asset: duplicate audio %q in manifest.json", entry.ID)
		}
		seen[entry.ID] = struct{}{}
	}
	return nil
}

func validateAssetPath(kind, name, path string) error {
	if path == "." || !fs.ValidPath(path) {
		return fmt.Errorf("asset: %s %q has invalid file path %q", kind, name, path)
	}
	return nil
}

func validateDynamicEntries(entries []dynamicEntry) error {
	for _, entry := range entries {
		if entry.W <= 0 || entry.H <= 0 {
			return fmt.Errorf("asset: dynamic texture %q has non-positive dimensions (%d x %d)", entry.Name, entry.W, entry.H)
		}
	}
	return nil
}

// statLayered returns metadata for the winning file layer without reading its
// bytes. Audio stays lazy, so Load uses this to validate declared clip sizes.
func statLayered(layers []fs.FS, path string) (fs.FileInfo, error) {
	for i := len(layers) - 1; i >= 0; i-- {
		info, err := fs.Stat(layers[i], path)
		if err == nil {
			return info, nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
	}
	return nil, fmt.Errorf("%w: %s", fs.ErrNotExist, path)
}

func readLayered(layers []fs.FS, path string, maxBytes int64) ([]byte, error) {
	for i := len(layers) - 1; i >= 0; i-- {
		info, err := fs.Stat(layers[i], path)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if err := validateFileSize(path, info.Size(), maxBytes); err != nil {
			return nil, err
		}

		data, err := fs.ReadFile(layers[i], path)
		if err == nil {
			if int64(len(data)) > maxBytes {
				return nil, fmt.Errorf("asset file %q exceeds limit %d bytes", path, maxBytes)
			}
			return data, nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
	}
	return nil, fmt.Errorf("%w: %s", fs.ErrNotExist, path)
}

func validateFileSize(path string, size, maxBytes int64) error {
	if size < 0 {
		return fmt.Errorf("asset file %q has invalid negative size %d", path, size)
	}
	if size > maxBytes {
		return fmt.Errorf("asset file %q exceeds limit %d bytes", path, maxBytes)
	}
	return nil
}
