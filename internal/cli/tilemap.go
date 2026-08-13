package cli

import (
	"bytes"
	"cmp"
	"encoding/xml"
	"fmt"
	"go/format"
	"go/token"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/tilemap"
	"github.com/lafriks/go-tiled"
	"github.com/spf13/cobra"
)

type tilemapImportFlags struct {
	output      string
	packageName string
	name        string
	cellSize    float64
	textures    []string
}

type tilemapTextureMapping struct {
	source string
	name   string
}

var tilemapImportOptions tilemapImportFlags

var tilemapCmd = &cobra.Command{
	Use:   "tilemap",
	Short: "tilemap tooling",
}

var tilemapImportCmd = &cobra.Command{
	Use:   "import <source.tmx>",
	Short: "compile a finite Tiled map into tilemap data",
	Args:  cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		return compileTilemap(args[0], tilemapImportOptions)
	},
}

func init() {
	tilemapImportCmd.Flags().StringVarP(&tilemapImportOptions.output, "output", "o", "", "generated Go output path")
	tilemapImportCmd.Flags().StringVar(&tilemapImportOptions.packageName, "package", "", "generated Go package")
	tilemapImportCmd.Flags().StringVar(&tilemapImportOptions.name, "name", "", "exported generated map name (default: source file name)")
	tilemapImportCmd.Flags().Float64Var(&tilemapImportOptions.cellSize, "cell-size", 1, "world size of one map cell")
	tilemapImportCmd.Flags().StringArrayVar(&tilemapImportOptions.textures, "texture", nil, "texture mapping SOURCE=MANIFEST_NAME (repeatable)")
	_ = tilemapImportCmd.MarkFlagRequired("output")
	_ = tilemapImportCmd.MarkFlagRequired("package")
	tilemapCmd.AddCommand(tilemapImportCmd)
	rootCmd.AddCommand(tilemapCmd)
}

func compileTilemap(source string, options tilemapImportFlags) error {
	if options.output == "" {
		return fmt.Errorf("tilemap import: --output is required")
	}
	if !token.IsIdentifier(options.packageName) {
		return fmt.Errorf("tilemap import: --package must be a valid Go identifier")
	}
	if options.cellSize <= 0 {
		return fmt.Errorf("tilemap import: --cell-size must be positive")
	}
	if options.name == "" {
		options.name = tilemapGeneratedName(filepath.Base(source))
	}
	if !validTilemapPublicName(options.name) {
		return fmt.Errorf("tilemap import: --name must be a valid exported Go identifier")
	}
	mappings, err := parseTilemapTextureMappings(options.textures)
	if err != nil {
		return err
	}

	mapData, err := tiled.LoadFile(source)
	if err != nil {
		if tiledUsesZstd(source) {
			return fmt.Errorf("tilemap import: zstd compression is unsupported; export CSV, gzip, or zlib")
		}
		return fmt.Errorf("tilemap import: load %q: %w", source, err)
	}
	raw, err := os.ReadFile(source)
	if err != nil {
		return fmt.Errorf("tilemap import: read %q: %w", source, err)
	}
	generated, err := generateTilemap(mapData, options, mappings, tilemapLayerOrders(raw))
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(options.output), 0o755); err != nil {
		return fmt.Errorf("tilemap import: create output directory: %w", err)
	}
	if err := os.WriteFile(options.output, generated, 0o644); err != nil {
		return fmt.Errorf("tilemap import: write %q: %w", options.output, err)
	}
	return nil
}

func tiledUsesZstd(source string) bool {
	data, err := os.ReadFile(source)
	return err == nil && bytes.Contains(data, []byte(`compression="zstd"`))
}

func parseTilemapTextureMappings(values []string) ([]tilemapTextureMapping, error) {
	mappings := make([]tilemapTextureMapping, 0, len(values))
	for _, value := range values {
		parts := strings.SplitN(value, "=", 2)
		if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
			return nil, fmt.Errorf("tilemap import: --texture expects SOURCE=MANIFEST_NAME, got %q", value)
		}
		source, name := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
		for _, mapping := range mappings {
			if mapping.source == source {
				return nil, fmt.Errorf("tilemap import: duplicate texture mapping %q", source)
			}
		}
		mappings = append(mappings, tilemapTextureMapping{source: source, name: name})
	}
	return mappings, nil
}

// tilemapLayerOrders maps each layer and object-group ID to its position in
// the map's document order, which is Tiled's draw order. The go-tiled split
// slices do not preserve that interleaving.
func tilemapLayerOrders(source []byte) map[uint32]int {
	orders := map[uint32]int{}
	decoder := xml.NewDecoder(bytes.NewReader(source))
	depth, next := 0, 0
	for {
		token, err := decoder.Token()
		if err != nil {
			return orders
		}
		switch element := token.(type) {
		case xml.StartElement:
			depth++
			if depth != 2 {
				continue
			}
			switch element.Name.Local {
			case "layer", "objectgroup", "imagelayer", "group":
			default:
				continue
			}
			for _, attr := range element.Attr {
				if attr.Name.Local == "id" {
					if id, err := strconv.ParseUint(attr.Value, 10, 32); err == nil {
						orders[uint32(id)] = next
					}
				}
			}
			next++
		case xml.EndElement:
			depth--
		}
	}
}

func generateTilemap(m *tiled.Map, options tilemapImportFlags, mappings []tilemapTextureMapping, orders map[uint32]int) ([]byte, error) {
	if m.Infinite {
		return nil, fmt.Errorf("tilemap import: infinite maps use Tiled chunks; export a finite map")
	}
	if m.Orientation != "orthogonal" && m.Orientation != "isometric" {
		return nil, fmt.Errorf("tilemap import: orientation %q is unsupported; use orthogonal or isometric", m.Orientation)
	}
	if m.RenderOrder != "" && m.RenderOrder != "right-down" {
		return nil, fmt.Errorf("tilemap import: render order %q is unsupported; use right-down", m.RenderOrder)
	}
	if len(m.ImageLayers) != 0 {
		return nil, fmt.Errorf("tilemap import: image layers are unsupported; use a tile layer")
	}
	if len(m.Groups) != 0 {
		return nil, fmt.Errorf("tilemap import: group layers are unsupported; flatten the group")
	}
	if m.Width <= 0 || m.Height <= 0 || m.TileWidth <= 0 || m.TileHeight <= 0 {
		return nil, fmt.Errorf("tilemap import: map dimensions and tile size must be positive")
	}

	var out bytes.Buffer
	fmt.Fprintln(&out, "// Code generated by arasy tilemap import; DO NOT EDIT.")
	fmt.Fprintln(&out)
	fmt.Fprintf(&out, "package %s\n\n", options.packageName)
	fmt.Fprintln(&out, "import (")
	fmt.Fprintln(&out, "\t\"github.com/dhannyell/arasy/math2d\"")
	fmt.Fprintln(&out, "\t\"github.com/dhannyell/arasy/render\"")
	fmt.Fprintln(&out, "\t\"github.com/dhannyell/arasy/scene\"")
	fmt.Fprintln(&out, "\t\"github.com/dhannyell/arasy/tilemap\"")
	fmt.Fprintln(&out, ")")
	fmt.Fprintln(&out)

	name := options.name
	fmt.Fprintf(&out, "// %sProperty is one custom property authored in Tiled.\n", name)
	fmt.Fprintf(&out, "type %sProperty struct {\n\tName string\n\tType string\n\tValue string\n}\n\n", name)
	fmt.Fprintf(&out, "// %sObject is raw object-layer data; games choose how to map it to entities.\n", name)
	fmt.Fprintf(&out, "// A tile object carries its atlas index in Tile with the texture of its\n")
	fmt.Fprintf(&out, "// tileset; Position is the tile's bottom-left corner, as Tiled anchors it.\n")
	fmt.Fprintf(&out, "type %sObject struct {\n\tGroup string\n\tID uint32\n\tName string\n\tClass string\n\tPosition math2d.Vec2\n\tSize math2d.Vec2\n\tRotation float64\n\tTile int16 // Atlas index into Texture; -1 when the object has no tile.\n\tTexture string\n\tFlip render.SpriteFlip\n\tProperties []%sProperty\n}\n\n", name, name)
	fmt.Fprintf(&out, "// %sLayer combines a tile layer with its atlas and Tiled parallax factor.\n", name)
	fmt.Fprintf(&out, "// Order is the layer's position in the map draw order; pass it as the render layer.\n")
	fmt.Fprintf(&out, "type %sLayer struct {\n\tName string\n\tOrder int\n\tLayer tilemap.Layer\n\tAtlas tilemap.Atlas\n\tParallax math2d.Vec2\n}\n\n", name)
	fmt.Fprintf(&out, "// %sTileObjectGroup holds one object group's visible tile objects, draw-ready\n", name)
	fmt.Fprintf(&out, "// for tilemap.NewObjectView. Order places the group in the map draw order.\n")
	fmt.Fprintf(&out, "type %sTileObjectGroup struct {\n\tName string\n\tOrder int\n\tTileObjects []tilemap.TileObject\n}\n\n", name)
	fmt.Fprintf(&out, "// %sMap is the generated map data. Collision is the solidity grid of the\n", name)
	fmt.Fprintf(&out, "// layer marked class=\"collision\" in Tiled; it is zero when the map has none.\n")
	fmt.Fprintf(&out, "type %sMap struct {\n\tLayers []%sLayer\n\tObjects []%sObject\n\tTileObjectGroups []%sTileObjectGroup\n\tCollision scene.CollisionGrid\n}\n\n", name, name, name, name)
	if m.Orientation == "isometric" {
		ratio := float64(m.TileWidth) / float64(m.TileHeight)
		fmt.Fprintf(&out, "// %sSuggestedIsoViewRatio is the map's tile width divided by its tile height, for a matching Camera.View.\n", name)
		fmt.Fprintf(&out, "const %sSuggestedIsoViewRatio = %s\n\n", name, tilemapFloat(ratio))
	}
	fmt.Fprintf(&out, "// New%sMap builds the map with texture names resolved by texture.\n", name)
	fmt.Fprintf(&out, "func New%sMap(texture func(string) render.TextureID) %sMap {\n", name, name)
	fmt.Fprintf(&out, "\treturn %sMap{\n\t\tLayers: []%sLayer{\n", name, name)

	var collision *tiled.Layer
	for index, layer := range m.Layers {
		if layer.Class == "collision" {
			if collision != nil {
				return nil, fmt.Errorf("tilemap import: layers %q and %q are both class=\"collision\"; keep at most one", collision.Name, layer.Name)
			}
			collision = layer
			continue
		}
		order := tilemapOrder(orders, layer.ID, index)
		if err := writeTilemapLayer(&out, layer, order, m, options.cellSize, mappings); err != nil {
			return nil, err
		}
	}
	fmt.Fprintln(&out, "\t\t},")
	fmt.Fprintf(&out, "\t\tObjects: []%sObject{\n", name)
	for _, group := range m.ObjectGroups {
		if err := validateTilemapObjectGroup(group); err != nil {
			return nil, err
		}
		for _, object := range group.Objects {
			if err := writeTilemapObject(&out, object, group.Name, group.OffsetX, group.OffsetY, m, options.cellSize, mappings, name); err != nil {
				return nil, err
			}
		}
	}
	fmt.Fprintln(&out, "\t\t},")
	fmt.Fprintf(&out, "\t\tTileObjectGroups: []%sTileObjectGroup{\n", name)
	for index, group := range m.ObjectGroups {
		if !group.Visible {
			continue
		}
		var objects bytes.Buffer
		for _, object := range tilemapObjectsInDrawOrder(group) {
			if err := writeTilemapTileObject(&objects, object, group.OffsetX, group.OffsetY, m, options.cellSize, mappings); err != nil {
				return nil, err
			}
		}
		if objects.Len() == 0 {
			continue
		}
		order := tilemapOrder(orders, group.ID, len(m.Layers)+index)
		fmt.Fprintf(&out, "\t\t\t{Name: %q, Order: %d, TileObjects: []tilemap.TileObject{\n", group.Name, order)
		out.Write(objects.Bytes())
		fmt.Fprintln(&out, "\t\t\t}},")
	}
	fmt.Fprintln(&out, "\t\t},")
	if collision != nil {
		if err := writeTilemapCollision(&out, collision, m, options.cellSize); err != nil {
			return nil, err
		}
	}
	fmt.Fprintln(&out, "\t}")
	fmt.Fprintln(&out, "}")

	generated, err := format.Source(out.Bytes())
	if err != nil {
		return nil, fmt.Errorf("tilemap import: format generated Go: %w", err)
	}
	return generated, nil
}

func validateTilemapObjectGroup(group *tiled.ObjectGroup) error {
	if group.Opacity != 1 || group.ParallaxX != 1 || group.ParallaxY != 1 || group.Mode != "" && group.Mode != "normal" {
		return fmt.Errorf("tilemap import: object layer %q uses opacity, parallax, or blend mode outside the v1 subset", group.Name)
	}
	if group.DrawOrder != "index" && group.DrawOrder != "topdown" {
		return fmt.Errorf("tilemap import: object layer %q uses unsupported draw order %q", group.Name, group.DrawOrder)
	}
	return nil
}

func tilemapObjectsInDrawOrder(group *tiled.ObjectGroup) []*tiled.Object {
	objects := slices.Clone(group.Objects)
	if group.DrawOrder == "topdown" {
		slices.SortStableFunc(objects, func(a, b *tiled.Object) int {
			return cmp.Compare(a.Y, b.Y)
		})
	}
	return objects
}

// tilemapOrder returns the document draw order of a layer ID, or fallback for
// maps old enough to lack layer IDs.
func tilemapOrder(orders map[uint32]int, id uint32, fallback int) int {
	if order, ok := orders[id]; ok {
		return order
	}
	return fallback
}

func writeTilemapLayer(out *bytes.Buffer, layer *tiled.Layer, order int, m *tiled.Map, cellSize float64, mappings []tilemapTextureMapping) error {
	if len(layer.Tiles) != m.Width*m.Height {
		return fmt.Errorf("tilemap import: layer %q has %d tiles, want %d", layer.Name, len(layer.Tiles), m.Width*m.Height)
	}
	if !layer.Visible || layer.Opacity != 1 || layer.Mode != "" && layer.Mode != "normal" {
		return fmt.Errorf("tilemap import: layer %q uses visibility, opacity, or blend mode outside the v1 subset; make it visible or mark class=\"collision\" for a data layer", layer.Name)
	}

	var tileset *tiled.Tileset
	for _, tile := range layer.Tiles {
		if tile == nil || tile.Nil {
			continue
		}
		if tileset == nil {
			tileset = tile.Tileset
			continue
		}
		if tileset != tile.Tileset {
			return fmt.Errorf("tilemap import: layer %q uses more than one tileset; split it into one layer per tileset", layer.Name)
		}
	}
	if tileset == nil {
		return fmt.Errorf("tilemap import: layer %q is empty and has no tileset", layer.Name)
	}
	if tileset.Image == nil || tileset.Image.Source == "" || tileset.Columns <= 0 || tileset.TileWidth <= 0 || tileset.TileHeight <= 0 {
		return fmt.Errorf("tilemap import: layer %q needs one grid tileset with an image, columns, and tile size", layer.Name)
	}

	textureName := tilemapTextureName(tileset.Image.Source, mappings)
	cellX := cellSize / float64(m.TileWidth)
	cellY := cellSize / float64(m.TileHeight)
	drawX := float64(tileset.TileWidth) * cellX
	drawY := float64(tileset.TileHeight) * cellY
	offsetX, offsetY := 0.0, 0.0
	if tileset.TileOffset != nil {
		offsetX = float64(tileset.TileOffset.X) * cellX
		offsetY = float64(tileset.TileOffset.Y) * cellY
	}

	animations, err := tilemapAnimations(tileset)
	if err != nil {
		return fmt.Errorf("%w (layer %q)", err, layer.Name)
	}
	if len(animations) > 0 {
		// The renderer advances flip-books through the raw texture grid; a
		// margin, spacing, or partial-width column count would desync frames.
		if tileset.Margin != 0 || tileset.Spacing != 0 {
			return fmt.Errorf("tilemap import: layer %q animates tiles in a tileset with margin or spacing; re-pack the atlas without gaps", layer.Name)
		}
		if tileset.Image.Width != tileset.Columns*tileset.TileWidth {
			return fmt.Errorf("tilemap import: layer %q animates tiles but the tileset columns do not span the image width", layer.Name)
		}
	}

	tiles := make([]int16, len(layer.Tiles))
	flips := make([]uint8, len(layer.Tiles))
	rotations := make([]float64, len(layer.Tiles))
	hasFlips, hasRotations := false, false
	for i, tile := range layer.Tiles {
		tiles[i] = -1
		if tile == nil || tile.Nil {
			continue
		}
		if tile.ID > math.MaxInt16 {
			return fmt.Errorf("tilemap import: layer %q tile %d exceeds int16 atlas index capacity", layer.Name, tile.ID)
		}
		flip, rotation := tiledTileTransform(tile.HorizontalFlip, tile.VerticalFlip, tile.DiagonalFlip)
		tiles[i] = int16(tile.ID)
		flips[i] = uint8(flip)
		rotations[i] = rotation
		hasFlips = hasFlips || flip != 0
		hasRotations = hasRotations || rotation != 0
	}

	fmt.Fprintf(out, "\t\t\t{Name: %q, Order: %d, Layer: tilemap.Layer{Tiles: []int16{", layer.Name, order)
	writeInt16s(out, tiles)
	fmt.Fprintf(out, "}, W: %d, H: %d, Origin: ", m.Width, m.Height)
	writeVec(out, layer.OffsetX*cellX, layer.OffsetY*cellY)
	fmt.Fprint(out, ", CellSize: ")
	writeVec(out, cellSize, cellSize)
	if hasFlips {
		fmt.Fprint(out, ", Flips: []render.SpriteFlip{")
		writeFlips(out, flips)
		fmt.Fprint(out, "}")
	}
	if hasRotations {
		fmt.Fprint(out, ", Rotations: []float64{")
		writeFloat64s(out, rotations)
		fmt.Fprint(out, "}")
	}
	fmt.Fprint(out, "}, Atlas: tilemap.Atlas{Texture: texture(")
	fmt.Fprintf(out, "%q), TileSize: ", textureName)
	writeVec(out, float64(tileset.TileWidth), float64(tileset.TileHeight))
	fmt.Fprintf(out, ", Columns: %d, DrawSize: ", tileset.Columns)
	writeVec(out, drawX, drawY)
	fmt.Fprint(out, ", Offset: ")
	writeVec(out, offsetX, offsetY)
	fmt.Fprint(out, ", Spacing: ")
	writeVec(out, float64(tileset.Spacing), float64(tileset.Spacing))
	fmt.Fprint(out, ", Margin: ")
	writeVec(out, float64(tileset.Margin), float64(tileset.Margin))
	if len(animations) > 0 {
		fmt.Fprint(out, ", Animations: []tilemap.Animation{")
		for _, animation := range animations {
			fmt.Fprintf(out, "{Tile: %d, Frames: %d, Rate: %s},", animation.tile, animation.frames, tilemapFloat32(animation.rate))
		}
		fmt.Fprint(out, "}")
	}
	fmt.Fprint(out, "}, Parallax: ")
	writeVec32(out, layer.ParallaxX, layer.ParallaxY)
	fmt.Fprintln(out, "},")
	return nil
}

// writeTilemapCollision emits the solidity grid of the collision layer: any
// non-empty cell is solid. The layer never draws, so visibility, opacity and
// tileset images are ignored; only its offset places the grid in the world.
func writeTilemapCollision(out *bytes.Buffer, layer *tiled.Layer, m *tiled.Map, cellSize float64) error {
	if len(layer.Tiles) != m.Width*m.Height {
		return fmt.Errorf("tilemap import: collision layer %q has %d tiles, want %d", layer.Name, len(layer.Tiles), m.Width*m.Height)
	}
	if layer.ParallaxX != 1 || layer.ParallaxY != 1 {
		return fmt.Errorf("tilemap import: collision layer %q uses parallax; collision cells do not move with the camera", layer.Name)
	}

	wordsPerRow := (m.Width + 63) / 64
	words := make([]uint64, wordsPerRow*m.Height)
	for i, tile := range layer.Tiles {
		if tile == nil || tile.Nil {
			continue
		}
		x, y := i%m.Width, i/m.Width
		words[y*wordsPerRow+x/64] |= 1 << uint(x%64)
	}

	cellX := cellSize / float64(m.TileWidth)
	cellY := cellSize / float64(m.TileHeight)
	fmt.Fprint(out, "\t\tCollision: scene.CollisionGrid{Words: []uint64{")
	for _, word := range words {
		fmt.Fprintf(out, "%#x,", word)
	}
	fmt.Fprintf(out, "}, W: %d, H: %d, Origin: ", m.Width, m.Height)
	writeVec(out, layer.OffsetX*cellX, layer.OffsetY*cellY)
	fmt.Fprint(out, ", CellSize: ")
	writeVec(out, cellSize, cellSize)
	fmt.Fprintln(out, "},")
	return nil
}

// tilemapAnimationEntry is one validated flip-book, ready to emit.
type tilemapAnimationEntry struct {
	tile   int16
	frames uint16
	rate   float32
}

// tilemapAnimations collects the flip-book table of one tileset in tile-ID
// order, so generation stays deterministic regardless of authoring order.
func tilemapAnimations(tileset *tiled.Tileset) ([]tilemapAnimationEntry, error) {
	var animations []tilemapAnimationEntry
	for _, tile := range tileset.Tiles {
		if len(tile.Animation) == 0 {
			continue
		}
		if tile.ID > math.MaxInt16 {
			return nil, fmt.Errorf("tilemap import: animated tile %d exceeds int16 atlas index capacity", tile.ID)
		}
		frames, rate, err := tiledFlipBook(tile)
		if err != nil {
			return nil, err
		}
		animations = append(animations, tilemapAnimationEntry{tile: int16(tile.ID), frames: frames, rate: rate})
	}
	slices.SortFunc(animations, func(a, b tilemapAnimationEntry) int {
		return cmp.Compare(a.tile, b.tile)
	})
	return animations, nil
}

// tiledFlipBook validates one Tiled animation against the engine flip-book
// subset and converts it: the frame count and the rate in frames per second.
// The subset requires frames on consecutive ascending tile IDs starting at
// the animated tile's own ID, all with one equal positive duration
// (milliseconds in Tiled). Anything else must error with an instruction to
// re-author the tileset, never drop data silently.
func tiledFlipBook(tile *tiled.TilesetTile) (uint16, float32, error) {
	if len(tile.Animation) > math.MaxUint16 {
		return 0, 0, fmt.Errorf("tilemap import: animated tile %d has %d frames, more than the flip-book maximum %d", tile.ID, len(tile.Animation), math.MaxUint16)
	}
	duration := tile.Animation[0].Duration
	if duration == 0 {
		return 0, 0, fmt.Errorf("tilemap import: animated tile %d has a zero-duration frame; give every frame one positive duration", tile.ID)
	}
	for i, frame := range tile.Animation {
		if frame.TileID != tile.ID+uint32(i) {
			return 0, 0, fmt.Errorf("tilemap import: animated tile %d frame %d uses tile %d; re-author the sequence on consecutive tiles starting at %d", tile.ID, i, frame.TileID, tile.ID)
		}
		if frame.Duration != duration {
			return 0, 0, fmt.Errorf("tilemap import: animated tile %d mixes frame durations (%dms and %dms); the flip-book advances at one constant rate", tile.ID, duration, frame.Duration)
		}
	}
	return uint16(len(tile.Animation)), 1000 / float32(duration), nil
}

func tiledTileTransform(horizontal, vertical, diagonal bool) (uint8, float64) {
	if !diagonal {
		return tileFlip(horizontal, vertical), 0
	}
	// Tiled applies diagonal first, then horizontal and vertical. For square
	// cells this canonical decomposition preserves that transform with the
	// renderer's sample flip followed by its center rotation.
	switch {
	case horizontal && vertical:
		return 1, math.Pi / 2
	case horizontal:
		return 0, math.Pi / 2
	case vertical:
		return 0, -math.Pi / 2
	default:
		return 2, math.Pi / 2
	}
}

func tileFlip(horizontal, vertical bool) uint8 {
	var flip uint8
	if horizontal {
		flip |= 1
	}
	if vertical {
		flip |= 2
	}
	return flip
}

func writeTilemapObject(out *bytes.Buffer, object *tiled.Object, group string, offsetX, offsetY float64, m *tiled.Map, cellSize float64, mappings []tilemapTextureMapping, name string) error {
	tile, texture, flip, err := tilemapObjectTile(object, m, mappings)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "\t\t\t{Group: %q, ID: %d, Name: %q, Class: %q, Position: ", group, object.ID, object.Name, object.Class)
	writeVec(out, (object.X+offsetX)*cellSize/float64(m.TileWidth), (object.Y+offsetY)*cellSize/float64(m.TileHeight))
	fmt.Fprint(out, ", Size: ")
	writeVec(out, object.Width*cellSize/float64(m.TileWidth), object.Height*cellSize/float64(m.TileHeight))
	fmt.Fprintf(out, ", Rotation: %s, Tile: %d", tilemapFloat(object.Rotation), tile)
	if tile >= 0 {
		fmt.Fprintf(out, ", Texture: %q, Flip: ", texture)
		writeFlips(out, []uint8{flip})
	} else {
		fmt.Fprint(out, ", ")
	}
	fmt.Fprintf(out, "Properties: []%sProperty{", name)
	for _, property := range object.Properties {
		fmt.Fprintf(out, "{Name: %q, Type: %q, Value: %q},", property.Name, property.Type, property.Value)
	}
	fmt.Fprintln(out, "}},")
	return nil
}

// tilemapObjectTile decodes a tile object's GID into the atlas index, texture
// name and source flip. Objects without a GID report tile -1.
func tilemapObjectTile(object *tiled.Object, m *tiled.Map, mappings []tilemapTextureMapping) (int16, string, uint8, error) {
	tile, texture, err := tilemapObjectLayerTile(object, m, mappings)
	if err != nil || tile == nil {
		return -1, "", 0, err
	}
	return int16(tile.ID), texture, tileFlip(tile.HorizontalFlip, tile.VerticalFlip), nil
}

// tilemapObjectLayerTile resolves and validates a tile object's GID. Objects
// without a GID return a nil tile.
func tilemapObjectLayerTile(object *tiled.Object, m *tiled.Map, mappings []tilemapTextureMapping) (*tiled.LayerTile, string, error) {
	if object.GID == 0 {
		return nil, "", nil
	}
	tile, err := m.TileGIDToTile(object.GID)
	if err != nil {
		return nil, "", fmt.Errorf("tilemap import: object %d: %w", object.ID, err)
	}
	if tile.DiagonalFlip {
		return nil, "", fmt.Errorf("tilemap import: object %d uses a diagonal tile flip; rotate the object instead", object.ID)
	}
	if tile.ID > math.MaxInt16 {
		return nil, "", fmt.Errorf("tilemap import: object %d tile %d exceeds int16 atlas index capacity", object.ID, tile.ID)
	}
	tileset := tile.Tileset
	if tileset == nil || tileset.Image == nil || tileset.Image.Source == "" || tileset.Columns <= 0 || tileset.TileWidth <= 0 || tileset.TileHeight <= 0 {
		return nil, "", fmt.Errorf("tilemap import: object %d needs one grid tileset with an image, columns, and tile size", object.ID)
	}
	return tile, tilemapTextureName(tileset.Image.Source, mappings), nil
}

// writeTilemapTileObject emits one draw-ready tilemap.TileObject. Tiled
// anchors a tile object at its bottom-left and rotates clockwise around that
// anchor; the emitted Dest center absorbs that pivot, since sprite instances
// rotate around their center.
func writeTilemapTileObject(out *bytes.Buffer, object *tiled.Object, offsetX, offsetY float64, m *tiled.Map, cellSize float64, mappings []tilemapTextureMapping) error {
	tile, texture, err := tilemapObjectLayerTile(object, m, mappings)
	if err != nil || tile == nil || !object.Visible {
		return err
	}
	tileset := tile.Tileset

	width := object.Width * cellSize / float64(m.TileWidth)
	height := object.Height * cellSize / float64(m.TileHeight)
	pivotX := (object.X + offsetX) * cellSize / float64(m.TileWidth)
	pivotY := (object.Y + offsetY) * cellSize / float64(m.TileHeight)
	rotation := object.Rotation * math.Pi / 180
	sin, cos := math.Sincos(rotation)
	centerX := pivotX + cos*(width/2) - sin*(-height/2)
	centerY := pivotY + sin*(width/2) + cos*(-height/2)

	// The same Atlas.SrcRect the runtime uses, evaluated at generation time.
	src := tilemap.Atlas{
		TileSize: math2d.Vec2{X: float64(tileset.TileWidth), Y: float64(tileset.TileHeight)},
		Columns:  tileset.Columns,
		Spacing:  math2d.Vec2{X: float64(tileset.Spacing), Y: float64(tileset.Spacing)},
		Margin:   math2d.Vec2{X: float64(tileset.Margin), Y: float64(tileset.Margin)},
	}.SrcRect(int16(tile.ID))

	fmt.Fprintf(out, "\t\t\t{Texture: texture(%q), Dest: math2d.NewRect(", texture)
	writeVec(out, centerX-width/2, centerY-height/2)
	fmt.Fprint(out, ", ")
	writeVec(out, width, height)
	fmt.Fprint(out, "), Src: math2d.NewRect(")
	writeVec(out, src.Min.X, src.Min.Y)
	fmt.Fprint(out, ", ")
	writeVec(out, float64(tileset.TileWidth), float64(tileset.TileHeight))
	fmt.Fprint(out, "), Flip: ")
	writeFlips(out, []uint8{tileFlip(tile.HorizontalFlip, tile.VerticalFlip)})
	fmt.Fprintf(out, "Rotation: %s", tilemapFloat(rotation))
	if definition, definitionErr := tileset.GetTilesetTile(tile.ID); definitionErr == nil && len(definition.Animation) > 0 {
		frames, rate, animationErr := tiledFlipBook(definition)
		if animationErr != nil {
			return animationErr
		}
		fmt.Fprintf(out, ", Animation: render.SpriteAnimation{Frames: %d, Rate: %s}", frames, tilemapFloat32(rate))
	}
	fmt.Fprintln(out, "},")
	return nil
}

func tilemapTextureName(source string, mappings []tilemapTextureMapping) string {
	for _, mapping := range mappings {
		if mapping.source == source {
			return mapping.name
		}
	}
	return strings.TrimSuffix(filepath.Base(source), filepath.Ext(source))
}

func writeInt16s(out *bytes.Buffer, values []int16) {
	for _, value := range values {
		fmt.Fprintf(out, "%d,", value)
	}
}

func writeFlips(out *bytes.Buffer, values []uint8) {
	for _, value := range values {
		switch value {
		case 0:
			fmt.Fprint(out, "render.FlipNone,")
		case 1:
			fmt.Fprint(out, "render.FlipX,")
		case 2:
			fmt.Fprint(out, "render.FlipY,")
		default:
			fmt.Fprint(out, "render.FlipX|render.FlipY,")
		}
	}
}

func writeFloat64s(out *bytes.Buffer, values []float64) {
	for _, value := range values {
		fmt.Fprintf(out, "%s,", tilemapFloat(value))
	}
}

func writeVec(out *bytes.Buffer, x, y float64) {
	fmt.Fprintf(out, "math2d.Vec2{X: %s, Y: %s}", tilemapFloat(x), tilemapFloat(y))
}

func writeVec32(out *bytes.Buffer, x, y float32) {
	fmt.Fprintf(out, "math2d.Vec2{X: %s, Y: %s}", tilemapFloat32(x), tilemapFloat32(y))
}

func tilemapFloat(value float64) string {
	return strconv.FormatFloat(value, 'g', -1, 64)
}

func tilemapFloat32(value float32) string {
	return strconv.FormatFloat(float64(value), 'g', 7, 64)
}

func validTilemapPublicName(value string) bool {
	if !token.IsIdentifier(value) {
		return false
	}
	for _, r := range value {
		return unicode.IsUpper(r)
	}
	return false
}

func tilemapGeneratedName(source string) string {
	base := strings.TrimSuffix(source, filepath.Ext(source))
	var out strings.Builder
	upperNext := true
	for _, r := range base {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			if upperNext {
				r = unicode.ToUpper(r)
				upperNext = false
			}
			out.WriteRune(r)
			continue
		}
		upperNext = true
	}
	name := out.String()
	if name == "" || unicode.IsDigit([]rune(name)[0]) {
		name = "Map" + name
	}
	return name
}
