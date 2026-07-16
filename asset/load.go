package asset

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"math"

	"github.com/dhannyell/frameforge/audio"
	"github.com/dhannyell/frameforge/math2d"
	"github.com/dhannyell/frameforge/particle"
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
	Effects  []effectEntry  `json:"effects"`
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

// effectEntry is the JSON shape of one particle effect declaration: authors
// write angles (direction, spread, rotation, spin) in degrees and
// [min, max]/[x, y]/[r, g, b, a] pairs. wobble_freq is a frequency in hertz,
// not an angle, and stays as authored. parseEffectEntry converts this shape
// into the resolved particle.EffectDecl (radians, math2d.Vec2, enums).
type effectEntry struct {
	Name         string    `json:"name"`
	Texture      string    `json:"texture"`
	Blend        string    `json:"blend"`
	Capacity     int       `json:"capacity"`
	EmitRate     float64   `json:"emit_rate"`
	BurstSize    int       `json:"burst_size"`
	Lifetime     []float64 `json:"lifetime"`
	Speed        []float64 `json:"speed"`
	Direction    float64   `json:"direction"`
	Spread       float64   `json:"spread"`
	Shape        string    `json:"shape"`
	EmitRadius   float64   `json:"emit_radius"`
	EmitExtent   []float64 `json:"emit_extent"`
	VelocityMode string    `json:"velocity_mode"`
	Gravity      []float64 `json:"gravity"`
	// Drag is a pointer so an omitted "drag" field is distinguishable from an
	// explicit 0, which would freeze every particle on the first frame (the
	// sim computes math.Pow(Drag, dt)). Nil defaults to 1 (no drag).
	Drag       *float64  `json:"drag"`
	SizeStart  []float64 `json:"size_start"`
	SizeEnd    []float64 `json:"size_end"`
	ColorStart []float64 `json:"color_start"`
	ColorEnd   []float64 `json:"color_end"`
	Spin       float64   `json:"spin"`
	Rotation   []float64 `json:"rotation"`
	WobbleAmp  float64   `json:"wobble_amp"`
	WobbleFreq float64   `json:"wobble_freq"`
	FadeIn     float64   `json:"fade_in"`
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
	effectByName := map[string]effectEntry{}
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
		if err := validateEffectEntries(mf.Effects); err != nil {
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
		for _, entry := range mf.Effects {
			effectByName[entry.Name] = entry
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
	effects := make(map[string]particle.EffectDecl, len(effectByName))
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
	for name, entry := range effectByName {
		decl, err := parseEffectEntry(entry, texByName)
		if err != nil {
			return Manifest{}, err
		}
		effects[name] = decl
	}

	manifest := newManifest(textures, nil, clips, fonts, effects)
	manifest.fsLayers = layers
	for name, s := range sprByName {
		texID, ok := manifest.textByName[s.Texture]
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

func validateEffectEntries(entries []effectEntry) error {
	seen := map[string]struct{}{}
	for _, entry := range entries {
		if entry.Name == "" {
			return errors.New("asset: effect name must not be empty in manifest.json")
		}
		if _, ok := seen[entry.Name]; ok {
			return fmt.Errorf("asset: duplicate effect %q in manifest.json", entry.Name)
		}
		seen[entry.Name] = struct{}{}
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

// parseEffectEntry resolves the JSON shape of a particle effect (snake_case
// fields, degrees, [min,max]/[x,y]/[r,g,b,a] pairs) into the engine-facing
// particle.EffectDecl (radians, math2d.Vec2, enums), validating every field
// that has a documented constraint.
func parseEffectEntry(entry effectEntry, textures map[string]textureEntry) (particle.EffectDecl, error) {
	name := entry.Name

	if entry.Capacity < 1 || entry.Capacity > particle.MaxCapacity {
		return particle.EffectDecl{}, fmt.Errorf("asset: effect %q has capacity %d outside [1, %d]", name, entry.Capacity, particle.MaxCapacity)
	}
	if entry.EmitRate < 0 {
		return particle.EffectDecl{}, fmt.Errorf("asset: effect %q has negative emit_rate %g", name, entry.EmitRate)
	}

	lifetime, err := effectFloatRange(name, "lifetime", entry.Lifetime)
	if err != nil {
		return particle.EffectDecl{}, err
	}
	if lifetime.Min <= 0 {
		return particle.EffectDecl{}, fmt.Errorf("asset: effect %q has lifetime.min %g, must be > 0", name, lifetime.Min)
	}

	speed, err := effectFloatRange(name, "speed", entry.Speed)
	if err != nil {
		return particle.EffectDecl{}, err
	}
	if len(entry.SizeStart) == 0 {
		return particle.EffectDecl{}, fmt.Errorf("asset: effect %q is missing required field %q", name, "size_start")
	}
	sizeStart, err := effectFloatRange(name, "size_start", entry.SizeStart)
	if err != nil {
		return particle.EffectDecl{}, err
	}
	if sizeStart.Min <= 0 || sizeStart.Max <= 0 {
		return particle.EffectDecl{}, fmt.Errorf("asset: effect %q has size_start %+v, both elements must be > 0", name, sizeStart)
	}
	sizeEnd, err := effectFloatRange(name, "size_end", entry.SizeEnd)
	if err != nil {
		return particle.EffectDecl{}, err
	}
	rotationDeg, err := effectFloatRange(name, "rotation", entry.Rotation)
	if err != nil {
		return particle.EffectDecl{}, err
	}
	gravity, err := effectVec2(name, "gravity", entry.Gravity)
	if err != nil {
		return particle.EffectDecl{}, err
	}
	emitExtent, err := effectVec2(name, "emit_extent", entry.EmitExtent)
	if err != nil {
		return particle.EffectDecl{}, err
	}
	colorStart, err := effectColor(name, "color_start", entry.ColorStart, [4]float64{1, 1, 1, 1})
	if err != nil {
		return particle.EffectDecl{}, err
	}
	colorEnd, err := effectColor(name, "color_end", entry.ColorEnd, [4]float64{1, 1, 1, 1})
	if err != nil {
		return particle.EffectDecl{}, err
	}
	drag := 1.0
	if entry.Drag != nil {
		drag = *entry.Drag
		if drag < 0 || drag > 1 {
			return particle.EffectDecl{}, fmt.Errorf("asset: effect %q has drag %g outside [0, 1]", name, drag)
		}
	}

	if entry.Texture != "" {
		if _, ok := textures[entry.Texture]; !ok {
			return particle.EffectDecl{}, fmt.Errorf("asset: effect %q references unknown texture %q", name, entry.Texture)
		}
	}

	blend, err := parseEffectBlend(name, entry.Blend)
	if err != nil {
		return particle.EffectDecl{}, err
	}
	shape, err := parseEffectShape(name, entry.Shape)
	if err != nil {
		return particle.EffectDecl{}, err
	}
	velMode, err := parseEffectVelocityMode(name, entry.VelocityMode)
	if err != nil {
		return particle.EffectDecl{}, err
	}

	switch shape {
	case particle.EmitDisc, particle.EmitRing:
		if entry.EmitRadius <= 0 {
			return particle.EffectDecl{}, fmt.Errorf("asset: effect %q has shape %q but emit_radius %g, must be > 0", name, entry.Shape, entry.EmitRadius)
		}
	case particle.EmitRect:
		if emitExtent.X <= 0 || emitExtent.Y <= 0 {
			return particle.EffectDecl{}, fmt.Errorf("asset: effect %q has shape \"rect\" but emit_extent %v, both elements must be > 0", name, entry.EmitExtent)
		}
	}

	if (velMode == particle.VelRadial || velMode == particle.VelTangential) && shape == particle.EmitPoint {
		return particle.EffectDecl{}, fmt.Errorf("asset: effect %q has velocity_mode %q, which requires a non-point shape", name, entry.VelocityMode)
	}

	if entry.FadeIn < 0 || entry.FadeIn >= 1 {
		return particle.EffectDecl{}, fmt.Errorf("asset: effect %q has fade_in %g, must be in [0, 1)", name, entry.FadeIn)
	}
	if entry.WobbleFreq < 0 {
		return particle.EffectDecl{}, fmt.Errorf("asset: effect %q has negative wobble_freq %g", name, entry.WobbleFreq)
	}

	return particle.EffectDecl{
		Name:       name,
		Texture:    entry.Texture,
		Blend:      blend,
		Capacity:   entry.Capacity,
		EmitRate:   entry.EmitRate,
		BurstSize:  entry.BurstSize,
		Lifetime:   lifetime,
		Speed:      speed,
		Direction:  effectDegToRad(entry.Direction),
		Spread:     effectDegToRad(entry.Spread),
		Shape:      shape,
		EmitRadius: entry.EmitRadius,
		EmitExtent: emitExtent,
		VelMode:    velMode,
		Gravity:    gravity,
		Drag:       drag,
		SizeStart:  sizeStart,
		SizeEnd:    sizeEnd,
		ColorStart: colorStart,
		ColorEnd:   colorEnd,
		Spin:       effectDegToRad(entry.Spin),
		Rotation:   particle.Range{Min: effectDegToRad(rotationDeg.Min), Max: effectDegToRad(rotationDeg.Max)},
		WobbleAmp:  entry.WobbleAmp,
		WobbleFreq: entry.WobbleFreq,
		FadeIn:     entry.FadeIn,
	}, nil
}

// effectDegToRad converts a manifest angle (authored in degrees) to radians.
func effectDegToRad(deg float64) float64 {
	return deg * math.Pi / 180
}

// effectFloatRange parses a [min, max] JSON pair. A missing field (nil or
// empty slice) resolves to the zero Range.
func effectFloatRange(effect, field string, arr []float64) (particle.Range, error) {
	if len(arr) == 0 {
		return particle.Range{}, nil
	}
	if len(arr) != 2 {
		return particle.Range{}, fmt.Errorf("asset: effect %q field %q must have exactly 2 elements [min, max], got %d", effect, field, len(arr))
	}
	return particle.Range{Min: arr[0], Max: arr[1]}, nil
}

// effectVec2 parses an [x, y] JSON pair. A missing field (nil or empty
// slice) resolves to the zero Vec2.
func effectVec2(effect, field string, arr []float64) (math2d.Vec2, error) {
	if len(arr) == 0 {
		return math2d.Vec2{}, nil
	}
	if len(arr) != 2 {
		return math2d.Vec2{}, fmt.Errorf("asset: effect %q field %q must have exactly 2 elements [x, y], got %d", effect, field, len(arr))
	}
	return math2d.Vec2{X: arr[0], Y: arr[1]}, nil
}

// effectColor parses an [r, g, b, a] JSON quadruple. A missing field (nil or
// empty slice) resolves to def, so an author who forgets color_start/color_end
// gets an opaque, visible default instead of a silently invisible effect.
func effectColor(effect, field string, arr []float64, def [4]float64) ([4]float64, error) {
	if len(arr) == 0 {
		return def, nil
	}
	if len(arr) != 4 {
		return [4]float64{}, fmt.Errorf("asset: effect %q field %q must have exactly 4 elements [r, g, b, a], got %d", effect, field, len(arr))
	}
	return [4]float64{arr[0], arr[1], arr[2], arr[3]}, nil
}

func parseEffectBlend(effect, value string) (particle.Blend, error) {
	switch value {
	case "", "alpha":
		return particle.BlendAlpha, nil
	case "additive":
		return particle.BlendAdditive, nil
	default:
		return 0, fmt.Errorf("asset: effect %q has unknown blend %q", effect, value)
	}
}

func parseEffectShape(effect, value string) (particle.EmitShape, error) {
	switch value {
	case "", "point":
		return particle.EmitPoint, nil
	case "disc":
		return particle.EmitDisc, nil
	case "rect":
		return particle.EmitRect, nil
	case "ring":
		return particle.EmitRing, nil
	default:
		return 0, fmt.Errorf("asset: effect %q has unknown shape %q", effect, value)
	}
}

func parseEffectVelocityMode(effect, value string) (particle.VelocityMode, error) {
	switch value {
	case "", "cone":
		return particle.VelCone, nil
	case "radial":
		return particle.VelRadial, nil
	case "tangential":
		return particle.VelTangential, nil
	default:
		return 0, fmt.Errorf("asset: effect %q has unknown velocity_mode %q", effect, value)
	}
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
