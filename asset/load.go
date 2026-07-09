package asset

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"

	"github.com/dhannyell/frameforge/audio"
	"github.com/dhannyell/frameforge/math2d"
)

type manifestFile struct {
	Version  int            `json:"version"`
	Textures []textureEntry `json:"textures"`
	Sprites  []spriteEntry  `json:"sprites"`
	Audio    []audioEntry   `json:"audio"`
}

type textureEntry struct {
	Name string `json:"name"`
	File string `json:"file"`
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

// Load reads manifest.json from the given FS layers and resolves every
// declared asset. Later layers take priority ("last wins"), at two levels:
//   - manifest entries: a texture/sprite name redeclared in a later layer
//     replaces the earlier declaration;
//   - file bytes: the file path of a winning declaration is read from the
//     LAST layer that contains it, so a mod layer can replace a PNG
//     without shipping its own manifest.json.
//
// A layer without manifest.json is valid (file-override-only layer), but at
// least one layer must declare one.
func Load(layers ...fs.FS) (Manifest, error) {
	if len(layers) == 0 {
		return Manifest{}, errors.New("asset: Load needs at least one FS layer")
	}

	texByName := map[string]textureEntry{}
	sprByName := map[string]spriteEntry{}
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
		if err := validateAudioEntries(mf.Audio); err != nil {
			return Manifest{}, err
		}
		for _, entry := range mf.Textures {
			texByName[entry.Name] = entry
		}
		for _, entry := range mf.Sprites {
			sprByName[entry.Name] = entry
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
	clips := make(map[audio.ClipID]audio.Clip, len(audioByID))
	for name, entry := range texByName {
		data, err := readLayered(layers, entry.File)
		if err != nil {
			return Manifest{}, fmt.Errorf("asset: texture %q: %w", name, err)
		}
		textures[name] = TextureDecl{Data: data}
	}
	for id, entry := range audioByID {
		// Audio bytes stay on disk: the host resolves and streams them (HTTP on
		// web, embed on wasm). Only confirm the winning layer has the file so a
		// bad path fails at load, not at first playback.
		if err := statLayered(layers, entry.File); err != nil {
			return Manifest{}, fmt.Errorf("asset: audio %q: %w", id, err)
		}
		clips[audio.ClipID(id)] = audio.Clip{ID: audio.ClipID(id), Path: entry.File}
	}

	manifest := newManifest(textures, nil, clips)
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

func validateAudioEntries(entries []audioEntry) error {
	seen := map[string]struct{}{}
	for _, entry := range entries {
		if _, ok := seen[entry.ID]; ok {
			return fmt.Errorf("asset: duplicate audio %q in manifest.json", entry.ID)
		}
		seen[entry.ID] = struct{}{}
	}
	return nil
}

// statLayered reports whether path exists in any layer (last wins), without
// reading its bytes — used to validate declarations the host loads lazily.
func statLayered(layers []fs.FS, path string) error {
	for i := len(layers) - 1; i >= 0; i-- {
		_, err := fs.Stat(layers[i], path)
		if err == nil {
			return nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	}
	return fmt.Errorf("%w: %s", fs.ErrNotExist, path)
}

func readLayered(layers []fs.FS, path string) ([]byte, error) {
	for i := len(layers) - 1; i >= 0; i-- {
		data, err := fs.ReadFile(layers[i], path)
		if err == nil {
			return data, nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
	}
	return nil, fmt.Errorf("%w: %s", fs.ErrNotExist, path)
}
