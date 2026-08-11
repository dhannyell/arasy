package tess

import (
	"encoding/binary"
	"math"
	"reflect"
	"testing"

	"github.com/dhannyell/arasy/math2d"
	"github.com/dhannyell/arasy/render"
)

const testEpsilon = 1e-5

func TestAppendRectFillUsesClipSpaceCorners(t *testing.T) {
	var mesh Mesh
	AppendRect(&mesh, testCtx(), render.DrawRect{
		Rect: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 50, Y: 100}),
		Fill: render.RGBA(1, 0, 0, 1),
	})

	if len(mesh.Vertices) != 6 {
		t.Fatalf("len(vertices) = %d, want 6", len(mesh.Vertices))
	}

	want := [][2]float32{
		{-1, 1},
		{0, 1},
		{0, -1},
		{-1, 1},
		{0, -1},
		{-1, -1},
	}
	for i, want := range want {
		assertVertexPosition(t, mesh.Vertices[i], want[0], want[1])
	}
}

func TestAppendCircleIsDeterministicAndWithinRadius(t *testing.T) {
	cmd := render.DrawCircle{
		Center: math2d.Vec2{X: 50, Y: 50},
		Radius: 10,
		Fill:   render.RGBA(0, 1, 0, 1),
	}

	var first Mesh
	var second Mesh
	AppendCircle(&first, testCtx(), cmd)
	AppendCircle(&second, testCtx(), cmd)

	if !reflect.DeepEqual(first.Vertices, second.Vertices) {
		t.Fatal("AppendCircle is not deterministic for identical input")
	}
	if len(first.Vertices) == 0 || len(first.Vertices)%3 != 0 {
		t.Fatalf("len(vertices) = %d, want non-zero multiple of 3", len(first.Vertices))
	}

	const clipRadius = 0.2
	for _, v := range first.Vertices {
		dist := math.Hypot(float64(v.X), float64(v.Y))
		if dist > clipRadius+testEpsilon {
			t.Fatalf("vertex distance from center = %v, want <= %v", dist, clipRadius)
		}
	}
}

func TestAppendCircleFillAvoidsCenterFanJunction(t *testing.T) {
	var mesh Mesh
	cmd := render.DrawCircle{
		Center: math2d.Vec2{X: 50, Y: 50},
		Radius: 10,
		Fill:   render.RGBA(1, 1, 1, 1),
	}
	AppendCircle(&mesh, testCtx(), cmd)

	n := circleSegments(cmd.Radius)
	if got, want := len(mesh.Vertices), (n-2)*3; got != want {
		t.Fatalf("len(vertices) = %d, want %d for boundary-anchored triangulation", got, want)
	}

	center := screenToClip(testCtx(), cmd.Center)
	for i, vertex := range mesh.Vertices {
		if vertex.X == center[0] && vertex.Y == center[1] {
			t.Fatalf("vertex %d is the circle center; fill must avoid a center fan junction", i)
		}
	}
}

func TestAppendLineUsesScreenSpaceWidth(t *testing.T) {
	var mesh Mesh
	AppendLine(&mesh, testCtx(), render.DrawLine{
		From:  math2d.Vec2{X: 10, Y: 50},
		To:    math2d.Vec2{X: 90, Y: 50},
		Width: 10,
		Color: render.RGBA(1, 1, 1, 1),
	})

	if len(mesh.Vertices) != 6 {
		t.Fatalf("len(vertices) = %d, want 6", len(mesh.Vertices))
	}

	minY := float32(math.Inf(1))
	maxY := float32(math.Inf(-1))
	for _, v := range mesh.Vertices {
		minY = min32(minY, v.Y)
		maxY = max32(maxY, v.Y)
	}

	assertFloat32(t, minY, -0.1)
	assertFloat32(t, maxY, 0.1)
}

func TestAppendPolygonConcaveFillMatchesArea(t *testing.T) {
	points := []math2d.Vec2{
		{X: 0, Y: 0},
		{X: 3, Y: 0},
		{X: 3, Y: 1},
		{X: 1, Y: 1},
		{X: 1, Y: 3},
		{X: 0, Y: 3},
	}

	tris := earClip(points)
	if len(tris) != 4 {
		t.Fatalf("len(tris) = %d, want 4", len(tris))
	}

	gotArea := 0.0
	for _, tri := range tris {
		gotArea += math.Abs(triangleArea(tri[0], tri[1], tri[2]))
	}

	wantArea := math.Abs(polygonSignedArea(points))
	assertFloat64(t, gotArea, wantArea)
}

func TestClipRestrictsGeneratedVertices(t *testing.T) {
	var mesh Mesh
	AppendRect(&mesh, testCtx(), render.DrawRect{
		BaseCommand: render.BaseCommand{
			Clip: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 50, Y: 100}),
		},
		Rect: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 100, Y: 100}),
		Fill: render.RGBA(1, 1, 1, 1),
	})

	if len(mesh.Vertices) == 0 {
		t.Fatal("clipped rect produced no vertices")
	}
	for _, v := range mesh.Vertices {
		if v.X > testEpsilon {
			t.Fatalf("vertex X = %v, want <= 0", v.X)
		}
	}
}

func TestColorsRemainStraightAlpha(t *testing.T) {
	color := render.RGBA(0.25, 0.5, 0.75, 0.5)
	var mesh Mesh
	AppendRect(&mesh, testCtx(), render.DrawRect{
		Rect: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 10, Y: 10}),
		Fill: color,
	})

	if len(mesh.Vertices) == 0 {
		t.Fatal("rect produced no vertices")
	}

	got := mesh.Vertices[0]
	assertFloat32(t, got.R, float32(color.R))
	assertFloat32(t, got.G, float32(color.G))
	assertFloat32(t, got.B, float32(color.B))
	assertFloat32(t, got.A, float32(color.A))
}

func TestBuildSortsSolidCommandsByLayer(t *testing.T) {
	first := render.RGBA(1, 0, 0, 1)
	second := render.RGBA(0, 0, 1, 1)
	var builder Builder
	runs := builder.Build(Frame{
		Camera: testCamera(),
		Commands: []render.Command{
			render.DrawRect{
				BaseCommand: render.BaseCommand{LayerValue: 10},
				Rect:        math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 10, Y: 10}),
				Fill:        first,
			},
			render.DrawRect{
				BaseCommand: render.BaseCommand{LayerValue: 0},
				Rect:        math2d.NewRect(math2d.Vec2{X: 20, Y: 20}, math2d.Vec2{X: 10, Y: 10}),
				Fill:        second,
			},
		},
	}, 100, 100, nil, nil)

	if len(runs) != 1 || runs[0].Kind != DrawRunSolid {
		t.Fatalf("runs = %+v, want one solid run", runs)
	}
	solid, _ := builder.Streams()
	if got := len(solid) / SolidVertexStride; got != 12 {
		t.Fatalf("len(vertices) = %d, want 12", got)
	}
	assertFloat32(t, math.Float32frombits(binary.LittleEndian.Uint32(solid[16:])), float32(second.B))
	assertFloat32(t, math.Float32frombits(binary.LittleEndian.Uint32(solid[6*SolidVertexStride+8:])), float32(first.R))
}

func TestBuildPassesKeepRunsAndStreamsInPassOrder(t *testing.T) {
	var builder Builder
	first := render.DrawRect{
		Rect: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 10, Y: 10}),
		Fill: render.RGBA(1, 0, 0, 1),
	}
	second := render.DrawRect{
		Rect: math2d.NewRect(math2d.Vec2{X: 20, Y: 20}, math2d.Vec2{X: 10, Y: 10}),
		Fill: render.RGBA(0, 0, 1, 1),
	}

	passes := builder.BuildPasses([]Frame{
		{Camera: testCamera(), Commands: []render.Command{first}},
		{Camera: testCamera(), Commands: []render.Command{second}},
	}, 100, 100, nil, nil)

	if len(passes) != 2 || len(passes[0].Runs) != 1 || len(passes[1].Runs) != 1 {
		t.Fatalf("passes = %+v, want two single-run passes", passes)
	}
	if passes[0].Runs[0].Pass != 0 || passes[1].Runs[0].Pass != 1 {
		t.Fatalf("run pass indices = %d, %d; want 0, 1", passes[0].Runs[0].Pass, passes[1].Runs[0].Pass)
	}
	if got, want := passes[1].Runs[0].ByteOffset, 6*SolidVertexStride; got != want {
		t.Fatalf("second pass byte offset = %d, want %d", got, want)
	}
	solid, _ := builder.Streams()
	if len(solid) == 0 {
		t.Fatal("Streams returned no values; expected shared solid stream")
	}
}

// TestBuildGroupsConsecutiveSameTextureDrawsAndSplitsOnAlternation confirms
// that Builder's run grouping is wired correctly into the DrawImage
// dispatch: consecutive draws of the same texture share one run, a different
// texture starts a new run, and vertex counts plus overall draw order are
// preserved across the split runs.
func TestBuildGroupsConsecutiveSameTextureDrawsAndSplitsOnAlternation(t *testing.T) {
	sizes := TexSizes{{}, {8, 8}, {8, 8}}
	dest := math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 10, Y: 10})

	var builder Builder
	runs := builder.Build(Frame{
		Camera: testCamera(),
		Commands: []render.Command{
			render.DrawImage{Texture: 1, Dest: dest},
			render.DrawImage{Texture: 1, Dest: dest},
			render.DrawImage{Texture: 2, Dest: dest},
			render.DrawImage{Texture: 1, Dest: dest},
		},
	}, 100, 100, nil, sizes)

	wantTextures := []render.TextureID{1, 2, 1}
	wantRecordCounts := []int{2, 1, 1}
	if len(runs) != len(wantTextures) {
		t.Fatalf("len(runs) = %d, want %d runs", len(runs), len(wantTextures))
	}
	for i, run := range runs {
		if run.Kind != DrawRunSprites || run.Texture != wantTextures[i] {
			t.Fatalf("runs[%d] = %+v, want sprite texture %d", i, run, wantTextures[i])
		}
		if run.VertexCount != wantRecordCounts[i] {
			t.Fatalf("sprite[%d] record count = %d, want %d", i, run.VertexCount, wantRecordCounts[i])
		}
	}
}

func TestAtlasAndAppendText(t *testing.T) {
	atlas := NewAtlas()
	if atlas.Image == nil {
		t.Fatal("atlas image is nil")
	}
	if !hasNonZeroAlpha(atlas.Image.Pix) {
		t.Fatal("atlas has no alpha pixels")
	}

	var mesh TextMesh
	AppendText(&mesh, testCtx(), render.DrawText{
		Position: math2d.Vec2{X: 50, Y: 50},
		Text:     "Hi",
		Color:    render.RGBA(1, 1, 1, 1),
		Size:     10,
		Align:    render.AlignCenter,
		Baseline: render.BaselineMiddle,
		Rotation: math.Pi / 2,
	}, atlas)

	if len(mesh.Vertices) != 12 {
		t.Fatalf("len(text vertices) = %d, want 12", len(mesh.Vertices))
	}
	for _, v := range mesh.Vertices {
		if v.U < 0 || v.U > 1 || v.V < 0 || v.V > 1 {
			t.Fatalf("UV out of range: %v, %v", v.U, v.V)
		}
	}
}

func TestBytesUseLittleEndianFloat32Layout(t *testing.T) {
	mesh := Mesh{Vertices: []Vertex{{
		X: -1,
		Y: 0.5,
		R: 0.25,
		G: 0.5,
		B: 0.75,
		A: 1,
	}}}

	got := Bytes(mesh)
	if len(got) != 24 {
		t.Fatalf("len(Bytes) = %d, want 24", len(got))
	}

	assertEncodedFloat32(t, got, 0, -1)
	assertEncodedFloat32(t, got, 4, 0.5)
	assertEncodedFloat32(t, got, 8, 0.25)
	assertEncodedFloat32(t, got, 20, 1)
}

func TestTextBytesUseLittleEndianFloat32Layout(t *testing.T) {
	mesh := TextMesh{Vertices: []TextVertex{{
		X: -1,
		Y: 0.5,
		R: 0.25,
		G: 0.5,
		B: 0.75,
		A: 1,
		U: 0.125,
		V: 0.875,
	}}}

	got := TextBytes(mesh)
	if len(got) != 32 {
		t.Fatalf("len(TextBytes) = %d, want 32", len(got))
	}

	assertEncodedFloat32(t, got, 0, -1)
	assertEncodedFloat32(t, got, 20, 1)
	assertEncodedFloat32(t, got, 24, 0.125)
	assertEncodedFloat32(t, got, 28, 0.875)
}

func testCtx() ctx {
	return ctx{
		cam: testCamera(),
		w:   100,
		h:   100,
	}
}

func testCamera() render.Camera {
	return render.Camera{
		World:    math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 100, Y: 100}),
		Viewport: math2d.NewRect(math2d.Vec2{}, math2d.Vec2{X: 100, Y: 100}),
		Aspect:   render.AspectStretch,
	}
}

func assertVertexPosition(t *testing.T, got Vertex, wantX, wantY float32) {
	t.Helper()
	assertFloat32(t, got.X, wantX)
	assertFloat32(t, got.Y, wantY)
}

func assertFloat32(t *testing.T, got, want float32) {
	t.Helper()
	if math.Abs(float64(got-want)) > testEpsilon {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func assertFloat64(t *testing.T, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > testEpsilon {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func assertEncodedFloat32(t *testing.T, got []byte, offset int, want float32) {
	t.Helper()
	bits := binary.LittleEndian.Uint32(got[offset : offset+4])
	value := math.Float32frombits(bits)
	assertFloat32(t, value, want)
}

func triangleArea(a, b, c math2d.Vec2) float64 {
	return cross2(b.Sub(a), c.Sub(a)) * 0.5
}

func hasNonZeroAlpha(pixels []byte) bool {
	for _, p := range pixels {
		if p != 0 {
			return true
		}
	}
	return false
}
