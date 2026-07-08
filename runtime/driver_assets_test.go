package runtime_test

import (
	"testing"
	"testing/fstest"

	"github.com/dhannyell/frameforge/asset"
	"github.com/dhannyell/frameforge/render"
	"github.com/dhannyell/frameforge/runtime"
)

type assetProbeGame struct {
	assets   render.TextureID
	renderID render.TextureID
}

func (g *assetProbeGame) Setup() runtime.Config {
	m, err := asset.Load(fstest.MapFS{
		"manifest.json":     {Data: []byte(`{"version":1,"textures":[{"name":"ball","file":"textures/ball.bin"}]}`)},
		"textures/ball.bin": {Data: []byte("dummy")},
	})
	if err != nil {
		panic(err)
	}
	return runtime.Config{Assets: m}
}

func (g *assetProbeGame) Init(ctx *runtime.UpdateContext) {}

func (g *assetProbeGame) Update(ctx *runtime.UpdateContext, dt float64) {
	g.assets = ctx.Assets.Texture("ball")
}

func (g *assetProbeGame) Render(ctx *runtime.RenderContext) []render.Command {
	g.renderID = ctx.Assets.Texture("ball")
	return nil
}

func TestDriverPassesAssetsToContexts(t *testing.T) {
	game := &assetProbeGame{}
	driver := runtime.NewDriver(game, testLogger())

	driver.Step()
	driver.Frame(0)

	if game.assets == 0 || game.renderID == 0 {
		t.Fatalf("asset ids = %d/%d, want non-zero", game.assets, game.renderID)
	}
	if game.assets != game.renderID {
		t.Fatalf("asset ids differ: update=%d render=%d", game.assets, game.renderID)
	}
}

type emptyAssetGame struct{}

func (g *emptyAssetGame) Setup() runtime.Config                              { return runtime.Config{} }
func (g *emptyAssetGame) Init(ctx *runtime.UpdateContext)                    {}
func (g *emptyAssetGame) Update(ctx *runtime.UpdateContext, dt float64)      {}
func (g *emptyAssetGame) Render(ctx *runtime.RenderContext) []render.Command { return nil }

func TestDriverZeroAssetsAreSafe(t *testing.T) {
	driver := runtime.NewDriver(&emptyAssetGame{}, testLogger())
	driver.Step()
}
