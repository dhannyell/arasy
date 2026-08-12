package cli

import (
	"bytes"
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
	if mapData.TileWidth != 32 || mapData.TileHeight != 16 || len(mapData.Tilesets) != 1 {
		t.Fatalf("isometric fixture = %#v, want 2:1 map with one tileset", mapData)
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
			_, err := generateTilemap(test.mapData, tilemapImportFlags{packageName: "fixture", name: "Fixture", cellSize: 1}, nil)
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
	generated, err := generateTilemap(mapData, tilemapImportFlags{packageName: "fixture", name: "Square", cellSize: 1}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(generated), "const SquareSuggestedIsoViewRatio = 1") {
		t.Fatalf("generated isometric ratio does not match square tiles:\n%s", generated)
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
