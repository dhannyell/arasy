package cli

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"image/png"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lafriks/go-tiled"
)

func TestOrthogonalFixturePinsAllTiledGIDTransforms(t *testing.T) {
	source := tilemapFixture("orthogonal-flips.tmx")
	mapData, err := tiled.LoadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	if len(mapData.Layers) != 2 || mapData.Layers[1].ParallaxX != 0.3 || len(mapData.ObjectGroups) != 1 {
		t.Fatalf("orthogonal fixture lost its two layers, parallax, or object group: %#v", mapData)
	}

	want := []struct {
		name     string
		flip     uint8
		rotation float64
	}{
		{name: "none"},
		{name: "horizontal", flip: 1},
		{name: "vertical", flip: 2},
		{name: "horizontal vertical", flip: 3},
		{name: "diagonal", flip: 2, rotation: math.Pi / 2},
		{name: "diagonal horizontal", rotation: math.Pi / 2},
		{name: "diagonal vertical", rotation: -math.Pi / 2},
		{name: "diagonal horizontal vertical", flip: 1, rotation: math.Pi / 2},
	}
	for index, test := range want {
		tile := mapData.Layers[0].Tiles[index]
		flip, rotation := tiledTileTransform(tile.HorizontalFlip, tile.VerticalFlip, tile.DiagonalFlip)
		if flip != test.flip || rotation != test.rotation {
			t.Fatalf("%s transform = (%d, %v), want (%d, %v)", test.name, flip, rotation, test.flip, test.rotation)
		}
	}

	assertGeneratedTilemapFixture(t, source, "orthogonal-flips.gen.go", tilemapImportFlags{
		packageName: "fixture", name: "OrthogonalFlips", cellSize: 2,
		textures: []string{"asymmetric.png=asymmetric"},
	})
}

func TestIsometricTallFixtureStaysGenerated(t *testing.T) {
	source := tilemapFixture("isometric-tall.tmx")
	mapData, err := tiled.LoadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	if mapData.TileWidth != 32 || mapData.TileHeight != 16 || len(mapData.Tilesets) != 1 || len(mapData.ObjectGroups) != 1 {
		t.Fatalf("isometric fixture = %#v, want 2:1 map with one tileset and one object group", mapData)
	}
	// Isometric object pixels divide by the tile height on both axes; the
	// generated fixture pins the converted positions byte-for-byte.
	if divX, divY := tilemapObjectPixelDivisors(mapData); divX != 16 || divY != 16 {
		t.Fatalf("isometric object divisors = (%v, %v), want tile height on both axes", divX, divY)
	}
	tileset := mapData.Tilesets[0]
	if tileset.TileOffset == nil || tileset.TileOffset.X != -16 || tileset.TileOffset.Y != -24 || len(tileset.WangSets) != 1 {
		t.Fatalf("isometric tileset = %#v, want tall offset tile with Wang terrain", tileset)
	}
	imageFile, err := os.Open(tilemapFixture("asymmetric.png"))
	if err != nil {
		t.Fatal(err)
	}
	defer imageFile.Close()
	config, err := png.DecodeConfig(imageFile)
	if err != nil || config.Width != 64 || config.Height != 64 {
		t.Fatalf("asymmetric fixture image = %v (%v), want 64x64 PNG", config, err)
	}

	assertGeneratedTilemapFixture(t, source, "isometric-tall.gen.go", tilemapImportFlags{
		packageName: "fixture", name: "IsometricTall", cellSize: 8,
		textures: []string{"asymmetric.png=asymmetric"},
	})
}

func assertGeneratedTilemapFixture(t *testing.T, source, generatedName string, options tilemapImportFlags) {
	t.Helper()
	options.output = filepath.Join(t.TempDir(), generatedName)
	if err := compileTilemap(source, options); err != nil {
		t.Fatal(err)
	}
	first, err := os.ReadFile(options.output)
	if err != nil {
		t.Fatal(err)
	}
	if err := compileTilemap(source, options); err != nil {
		t.Fatal(err)
	}
	second, err := os.ReadFile(options.output)
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Fatal("tilemap import is not deterministic")
	}
	want, err := os.ReadFile(tilemapFixture(generatedName))
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(want) {
		t.Fatalf("%s is stale; regenerate it with arasy tilemap import", generatedName)
	}
}

func tilemapFixture(name string) string { return filepath.Join("testdata", "tilemap", name) }

func TestTiledTileTransform(t *testing.T) {
	tests := []struct {
		name                           string
		horizontal, vertical, diagonal bool
		flip                           uint8
		rotation                       float64
	}{
		{name: "none"},
		{name: "horizontal", horizontal: true, flip: 1},
		{name: "vertical", vertical: true, flip: 2},
		{name: "both", horizontal: true, vertical: true, flip: 3},
		{name: "diagonal", diagonal: true, flip: 2, rotation: math.Pi / 2},
		{name: "horizontal diagonal", horizontal: true, diagonal: true, rotation: math.Pi / 2},
		{name: "vertical diagonal", vertical: true, diagonal: true, rotation: -math.Pi / 2},
		{name: "both diagonal", horizontal: true, vertical: true, diagonal: true, flip: 1, rotation: math.Pi / 2},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			flip, rotation := tiledTileTransform(test.horizontal, test.vertical, test.diagonal)
			if flip != test.flip || rotation != test.rotation {
				t.Fatalf("transform = (%d, %v), want (%d, %v)", flip, rotation, test.flip, test.rotation)
			}
		})
	}
}

func TestGenerateTilemapRejectsUnsupportedMaps(t *testing.T) {
	tests := []struct {
		name    string
		mapData *tiled.Map
		want    string
	}{
		{name: "infinite", mapData: &tiled.Map{Infinite: true}, want: "infinite"},
		{name: "hexagonal", mapData: &tiled.Map{Orientation: "hexagonal"}, want: "orientation"},
		{name: "render order", mapData: &tiled.Map{Orientation: "orthogonal", RenderOrder: "left-up"}, want: "render order"},
		{name: "image layer", mapData: &tiled.Map{Orientation: "orthogonal", ImageLayers: []*tiled.ImageLayer{{}}}, want: "image layers"},
		{name: "group", mapData: &tiled.Map{Orientation: "orthogonal", Groups: []*tiled.Group{{}}}, want: "group layers"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := generateTilemap(test.mapData, tilemapImportFlags{packageName: "fixture", name: "Fixture", cellSize: 1}, nil, nil)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestCompileTilemapLoadsExternalTileset(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "tiles.tsx"), []byte(`<?xml version="1.0" encoding="UTF-8"?>
<tileset name="tiles" tilewidth="16" tileheight="16" tilecount="1" columns="1"><image source="tiles.png" width="16" height="16"/></tileset>`), 0o644); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(dir, "external.tmx")
	if err := os.WriteFile(source, []byte(`<?xml version="1.0" encoding="UTF-8"?>
<map orientation="isometric" renderorder="right-down" width="1" height="1" tilewidth="16" tileheight="8"><tileset firstgid="1" source="tiles.tsx"/><layer name="ground" width="1" height="1"><data encoding="csv">1</data></layer></map>`), 0o644); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(dir, "external.gen.go")
	if err := compileTilemap(source, tilemapImportFlags{output: output, packageName: "fixture", name: "External", cellSize: 1}); err != nil {
		t.Fatal(err)
	}
	generated, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(generated), "const ExternalSuggestedIsoViewRatio = 2") {
		t.Fatalf("isometric import did not emit its camera-view guidance:\n%s", generated)
	}
}

func TestCompileTilemapDerivesSuggestedIsoViewRatio(t *testing.T) {
	mapData := &tiled.Map{Orientation: "isometric", Width: 1, Height: 1, TileWidth: 32, TileHeight: 32}
	mapData.Layers = []*tiled.Layer{{
		Name: "ground", Visible: true, Opacity: 1,
		Tiles: []*tiled.LayerTile{{ID: 0, Tileset: &tiled.Tileset{
			Columns: 1, TileWidth: 32, TileHeight: 32, Image: &tiled.Image{Source: "terrain.png"},
		}}},
	}}
	generated, err := generateTilemap(mapData, tilemapImportFlags{packageName: "fixture", name: "Square", cellSize: 1}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(generated), "const SquareSuggestedIsoViewRatio = 1") {
		t.Fatalf("generated isometric ratio does not match square tiles:\n%s", generated)
	}
}

// collisionTestLayer builds one hand-made 2x2 collision layer. Loaded maps get
// parallax 1 by default; hand-built layers must set it explicitly.
func collisionTestLayer(name string, visible bool) *tiled.Layer {
	solid := &tiled.LayerTile{ID: 0, Tileset: &tiled.Tileset{}}
	return &tiled.Layer{
		Name: name, Class: "collision", Visible: visible, Opacity: 1,
		ParallaxX: 1, ParallaxY: 1,
		Tiles: []*tiled.LayerTile{solid, nil, nil, solid},
	}
}

// TestGenerateTilemapEmitsCollisionGrid proves an invisible class="collision"
// layer becomes the solidity bitset instead of failing the visibility check.
func TestGenerateTilemapEmitsCollisionGrid(t *testing.T) {
	mapData := &tiled.Map{Orientation: "orthogonal", Width: 2, Height: 2, TileWidth: 16, TileHeight: 16}
	mapData.Layers = []*tiled.Layer{collisionTestLayer("walls", false)}

	generated, err := generateTilemap(mapData, tilemapImportFlags{packageName: "fixture", name: "Walls", cellSize: 4}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := "scene.CollisionGrid{Words: []uint64{0x1, 0x2}, W: 2, H: 2"
	if !strings.Contains(string(generated), want) {
		t.Fatalf("generated map lacks the collision grid %q:\n%s", want, generated)
	}
}

func TestGenerateTilemapRejectsTwoCollisionLayers(t *testing.T) {
	mapData := &tiled.Map{Orientation: "orthogonal", Width: 2, Height: 2, TileWidth: 16, TileHeight: 16}
	mapData.Layers = []*tiled.Layer{collisionTestLayer("walls", false), collisionTestLayer("water", true)}

	_, err := generateTilemap(mapData, tilemapImportFlags{packageName: "fixture", name: "Walls", cellSize: 4}, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "at most one") {
		t.Fatalf("error = %v, want single-collision-layer rejection", err)
	}
}

func TestGenerateTilemapRejectsCollisionParallax(t *testing.T) {
	layer := collisionTestLayer("walls", false)
	layer.ParallaxX = 0.5
	mapData := &tiled.Map{Orientation: "orthogonal", Width: 2, Height: 2, TileWidth: 16, TileHeight: 16}
	mapData.Layers = []*tiled.Layer{layer}

	_, err := generateTilemap(mapData, tilemapImportFlags{packageName: "fixture", name: "Walls", cellSize: 4}, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "parallax") {
		t.Fatalf("error = %v, want collision parallax rejection", err)
	}
}

// animatedTestMap builds one 1x1 hand-made map whose tileset animates tile 0
// with the given frames. Columns span the image width, as the subset requires.
func animatedTestMap(frames []*tiled.AnimationFrame) *tiled.Map {
	tileset := &tiled.Tileset{
		Columns: 2, TileWidth: 16, TileHeight: 16,
		Image: &tiled.Image{Source: "anim.png", Width: 32, Height: 16},
		Tiles: []*tiled.TilesetTile{{ID: 0, Animation: frames}},
	}
	mapData := &tiled.Map{Orientation: "orthogonal", Width: 1, Height: 1, TileWidth: 16, TileHeight: 16}
	mapData.Layers = []*tiled.Layer{{
		Name: "ground", Visible: true, Opacity: 1,
		Tiles: []*tiled.LayerTile{{ID: 0, Tileset: tileset}},
	}}
	return mapData
}

// TestGenerateTilemapEmitsAnimations pins the Tiled-to-engine conversion: two
// 100ms frames on consecutive IDs become Frames 2 at 10 frames per second.
func TestGenerateTilemapEmitsAnimations(t *testing.T) {
	mapData := animatedTestMap([]*tiled.AnimationFrame{
		{TileID: 0, Duration: 100},
		{TileID: 1, Duration: 100},
	})

	generated, err := generateTilemap(mapData, tilemapImportFlags{packageName: "fixture", name: "Anim", cellSize: 1}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := "Animations: []tilemap.Animation{{Tile: 0, Frames: 2, Rate: 10}}"
	if !strings.Contains(string(generated), want) {
		t.Fatalf("generated map lacks %q:\n%s", want, generated)
	}
}

func TestCompileTilemapPreservesObjectLayerSemantics(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "objects.tmx")
	data := `<?xml version="1.0" encoding="UTF-8"?>
<map orientation="orthogonal" renderorder="right-down" width="1" height="1" tilewidth="16" tileheight="16">
 <tileset firstgid="1" name="tiles" tilewidth="16" tileheight="16" tilecount="2" columns="2">
  <image source="tiles.png" width="32" height="16"/>
  <tile id="0"><animation><frame tileid="0" duration="100"/><frame tileid="1" duration="100"/></animation></tile>
 </tileset>
 <objectgroup id="1" name="props" offsetx="16" offsety="8" draworder="topdown">
  <object id="1" gid="1" x="32" y="32" width="16" height="16"/>
  <object id="2" gid="2" x="0" y="16" width="16" height="16"/>
 </objectgroup>
</map>`
	if err := os.WriteFile(source, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(dir, "objects.gen.go")
	if err := compileTilemap(source, tilemapImportFlags{output: output, packageName: "fixture", name: "Objects", cellSize: 1}); err != nil {
		t.Fatal(err)
	}
	generated, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	text := string(generated)
	first := strings.Index(text, "X: 16, Y: 0")
	second := strings.Index(text, "X: 0, Y: 0")
	if first < 0 || second < 0 || first >= second {
		t.Fatalf("topdown tile objects are not sorted by Y: first=%d second=%d\n%s", first, second, text)
	}
	if !strings.Contains(text, "Animation: render.SpriteAnimation{Frames: 2, Rate: 10}") {
		t.Fatalf("animated tile object lost its flip-book:\n%s", text)
	}
	if !strings.Contains(text, "Position: math2d.Vec2{X: 3, Y: 2.5}") {
		t.Fatalf("raw object position did not include the group offset:\n%s", text)
	}
}

// TestCompileTilemapRejectsIsometricTileObjects closes the v1 subset: Tiled
// draws isometric tile objects as unprojected billboards, which the projected
// ObjectView path cannot reproduce.
func TestCompileTilemapRejectsIsometricTileObjects(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "iso-objects.tmx")
	data := `<?xml version="1.0" encoding="UTF-8"?>
<map orientation="isometric" renderorder="right-down" width="1" height="1" tilewidth="32" tileheight="16">
 <tileset firstgid="1" name="tiles" tilewidth="32" tileheight="16" tilecount="1" columns="1">
  <image source="tiles.png" width="32" height="16"/>
 </tileset>
 <objectgroup id="1" name="props" draworder="topdown">
  <object id="1" gid="1" x="16" y="8" width="32" height="16"/>
 </objectgroup>
</map>`
	if err := os.WriteFile(source, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	err := compileTilemap(source, tilemapImportFlags{output: filepath.Join(dir, "out.go"), packageName: "fixture", name: "IsoObjects", cellSize: 1})
	if err == nil || !strings.Contains(err.Error(), "tile object on an isometric map") {
		t.Fatalf("error = %v, want isometric tile-object rejection", err)
	}
}

func TestValidateTilemapObjectGroupRejectsUnsupportedPresentation(t *testing.T) {
	tests := []struct {
		group       *tiled.ObjectGroup
		orientation string
	}{
		{group: &tiled.ObjectGroup{Name: "opacity", Opacity: 0.5, ParallaxX: 1, ParallaxY: 1, DrawOrder: "topdown"}, orientation: "orthogonal"},
		{group: &tiled.ObjectGroup{Name: "parallax", Opacity: 1, ParallaxX: 0.5, ParallaxY: 1, DrawOrder: "topdown"}, orientation: "orthogonal"},
		{group: &tiled.ObjectGroup{Name: "blend", Opacity: 1, ParallaxX: 1, ParallaxY: 1, Mode: "multiply", DrawOrder: "topdown"}, orientation: "orthogonal"},
		{group: &tiled.ObjectGroup{Name: "draw order", Opacity: 1, ParallaxX: 1, ParallaxY: 1, DrawOrder: "unknown"}, orientation: "orthogonal"},
		// Screen-pixel offsets have no answer in isometric object space.
		{group: &tiled.ObjectGroup{Name: "iso offset", Opacity: 1, ParallaxX: 1, ParallaxY: 1, DrawOrder: "topdown", OffsetX: 8}, orientation: "isometric"},
	}
	for _, test := range tests {
		if err := validateTilemapObjectGroup(test.group, test.orientation); err == nil {
			t.Fatalf("object group %q must reject unsupported presentation", test.group.Name)
		}
	}
	ortho := &tiled.ObjectGroup{Name: "ortho offset", Opacity: 1, ParallaxX: 1, ParallaxY: 1, DrawOrder: "topdown", OffsetX: 8}
	if err := validateTilemapObjectGroup(ortho, "orthogonal"); err != nil {
		t.Fatalf("orthogonal group offsets stay in the subset: %v", err)
	}
}

// TestGenerateTilemapRejectsIrregularAnimations closes the v1 subset: frame
// sequences the flip-book cannot represent must error with an instruction,
// never silently drop.
func TestGenerateTilemapRejectsIrregularAnimations(t *testing.T) {
	tests := []struct {
		name   string
		frames []*tiled.AnimationFrame
	}{
		{"non-consecutive IDs", []*tiled.AnimationFrame{{TileID: 0, Duration: 100}, {TileID: 2, Duration: 100}}},
		{"does not start at own ID", []*tiled.AnimationFrame{{TileID: 1, Duration: 100}, {TileID: 2, Duration: 100}}},
		{"unequal durations", []*tiled.AnimationFrame{{TileID: 0, Duration: 100}, {TileID: 1, Duration: 200}}},
		{"zero duration", []*tiled.AnimationFrame{{TileID: 0, Duration: 0}, {TileID: 1, Duration: 0}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := generateTilemap(animatedTestMap(test.frames), tilemapImportFlags{packageName: "fixture", name: "Anim", cellSize: 1}, nil, nil)
			if err == nil {
				t.Fatal("irregular animation must fail the import")
			}
		})
	}
}

func TestCompileTilemapRejectsMissingExternalTileset(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "missing.tmx")
	data := `<?xml version="1.0" encoding="UTF-8"?><map orientation="orthogonal" width="1" height="1" tilewidth="16" tileheight="16"><tileset firstgid="1" source="missing.tsx"/><layer name="ground" width="1" height="1"><data encoding="csv">1</data></layer></map>`
	if err := os.WriteFile(source, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	err := compileTilemap(source, tilemapImportFlags{output: filepath.Join(dir, "out.go"), packageName: "fixture", name: "Missing", cellSize: 1})
	if err == nil || !strings.Contains(err.Error(), "missing.tsx") {
		t.Fatalf("error = %v, want missing external tileset path", err)
	}
}

func TestCompileTilemapRejectsZstd(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "zstd.tmx")
	data := `<?xml version="1.0" encoding="UTF-8"?><map orientation="orthogonal" width="1" height="1" tilewidth="16" tileheight="16"><tileset firstgid="1" name="tiles" tilewidth="16" tileheight="16" tilecount="1" columns="1"><image source="tiles.png" width="16" height="16"/></tileset><layer name="ground" width="1" height="1"><data encoding="base64" compression="zstd">AAAAAA==</data></layer></map>`
	if err := os.WriteFile(source, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	err := compileTilemap(source, tilemapImportFlags{output: filepath.Join(dir, "out.go"), packageName: "fixture", name: "Zstd", cellSize: 1})
	if err == nil || !strings.Contains(err.Error(), "zstd compression") {
		t.Fatalf("error = %v, want clear zstd rejection", err)
	}
}

func TestCompileTilemapAcceptsBase64Compression(t *testing.T) {
	for _, compression := range []string{"gzip", "zlib"} {
		t.Run(compression, func(t *testing.T) {
			var encoded bytes.Buffer
			var writer interface {
				Write([]byte) (int, error)
				Close() error
			}
			if compression == "gzip" {
				writer = gzip.NewWriter(&encoded)
			} else {
				writer = zlib.NewWriter(&encoded)
			}
			var gid [4]byte
			binary.LittleEndian.PutUint32(gid[:], 1)
			if _, err := writer.Write(gid[:]); err != nil {
				t.Fatal(err)
			}
			if err := writer.Close(); err != nil {
				t.Fatal(err)
			}

			dir := t.TempDir()
			source := filepath.Join(dir, compression+".tmx")
			data := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?><map orientation="orthogonal" renderorder="right-down" width="1" height="1" tilewidth="16" tileheight="16"><tileset firstgid="1" name="tiles" tilewidth="16" tileheight="16" tilecount="1" columns="1"><image source="tiles.png" width="16" height="16"/></tileset><layer name="ground" width="1" height="1"><data encoding="base64" compression="%s">%s</data></layer></map>`, compression, base64.StdEncoding.EncodeToString(encoded.Bytes()))
			if err := os.WriteFile(source, []byte(data), 0o644); err != nil {
				t.Fatal(err)
			}
			output := filepath.Join(dir, "out.go")
			if err := compileTilemap(source, tilemapImportFlags{output: output, packageName: "fixture", name: "Compressed", cellSize: 1}); err != nil {
				t.Fatal(err)
			}
			generated, err := os.ReadFile(output)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Contains(generated, []byte("Tiles: []int16{0}")) {
				t.Fatalf("compressed tile was not decoded:\n%s", generated)
			}
		})
	}
}

func TestWriteTilemapLayerRejectsMultipleTilesets(t *testing.T) {
	first := &tiled.Tileset{Columns: 1, TileWidth: 16, TileHeight: 16, Image: &tiled.Image{Source: "first.png"}}
	second := &tiled.Tileset{Columns: 1, TileWidth: 16, TileHeight: 16, Image: &tiled.Image{Source: "second.png"}}
	layer := &tiled.Layer{Opacity: 1, Visible: true, Tiles: []*tiled.LayerTile{{ID: 0, Tileset: first}, {ID: 0, Tileset: second}}}
	var out bytes.Buffer
	err := writeTilemapLayer(&out, layer, 0, &tiled.Map{Width: 2, Height: 1, TileWidth: 16, TileHeight: 16}, 1, nil)
	if err == nil || !strings.Contains(err.Error(), "more than one tileset") {
		t.Fatalf("error = %v, want multiple-tileset rejection", err)
	}
}

// TestWriteTilemapLayerRejectsDiagonalOnNonSquareDraw pins the guard: the
// diagonal decomposition is a 90-degree rotation of the drawn rect, valid
// only when that rect is square.
func TestWriteTilemapLayerRejectsDiagonalOnNonSquareDraw(t *testing.T) {
	tileset := &tiled.Tileset{Columns: 1, TileWidth: 16, TileHeight: 32, Image: &tiled.Image{Source: "tall.png"}}
	layer := &tiled.Layer{Opacity: 1, Visible: true, Tiles: []*tiled.LayerTile{{ID: 0, Tileset: tileset, DiagonalFlip: true}}}
	var out bytes.Buffer
	err := writeTilemapLayer(&out, layer, 0, &tiled.Map{Width: 1, Height: 1, TileWidth: 16, TileHeight: 16}, 1, nil)
	if err == nil || !strings.Contains(err.Error(), "diagonal flip") {
		t.Fatalf("error = %v, want non-square diagonal rejection", err)
	}
}

func TestOutpostMapStaysGenerated(t *testing.T) {
	root := filepath.Join("..", "..")
	source := filepath.Join(root, "examples", "outpost", "assets", "outpost.tmx")
	wantPath := filepath.Join(root, "examples", "outpost", "outpost_map.gen.go")
	output := filepath.Join(t.TempDir(), "outpost_map.gen.go")
	options := tilemapImportFlags{
		output: output, packageName: "outpost", name: "Outpost", cellSize: 8,
		textures: []string{"terrain_atlas.png=terrain_atlas"},
	}
	if err := compileTilemap(source, options); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(wantPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Fatalf("%s is stale; regenerate it with arasy tilemap import", wantPath)
	}
}

func TestEnginePackagesDoNotDependOnTiled(t *testing.T) {
	root := filepath.Join("..", "..")
	cmd := exec.Command("go", "list", "-deps", "./render/...", "./scene/...", "./tilemap/...", "./runtime/...")
	cmd.Dir = root
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go list -deps: %v\n%s", err, output)
	}
	if bytes.Contains(output, []byte("github.com/lafriks/go-tiled")) {
		t.Fatalf("engine dependency graph includes go-tiled:\n%s", output)
	}
}
