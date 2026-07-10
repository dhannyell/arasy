package canvas

import (
	"bytes"
	"image"
	"image/png"
	"net/http"
	"strconv"
	"strings"

	"github.com/dhannyell/frameforge/asset"
	"github.com/dhannyell/frameforge/render"
	"github.com/dhannyell/frameforge/runtime"
)

type textureUpdateDTO struct {
	Type      string `json:"type"`
	TextureID uint32 `json:"textureID"`
	Version   int    `json:"version"`
	URL       string `json:"url"`
}

type textureUpdateListDTO struct {
	Type    string             `json:"type"`
	Updates []textureUpdateDTO `json:"updates"`
}

func (h *hub) handleTextureUpdates(updates []runtime.TextureUpdate) {
	if len(updates) == 0 {
		return
	}
	h.dynamicTextureMu.Lock()
	defer h.dynamicTextureMu.Unlock()
	for _, u := range updates {
		if u.Image == nil {
			continue
		}
		h.textureVersion[u.ID]++
		var buf bytes.Buffer
		if err := png.Encode(&buf, u.Image); err != nil {
			continue
		}
		h.dynamicTextureData[u.ID] = buf.Bytes()
	}
}

func (h *hub) handleDynamicTexture(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimPrefix(r.URL.Path, "/assets/dynamic/")
	if idx := strings.IndexByte(idStr, '?'); idx >= 0 {
		idStr = idStr[:idx]
	}
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	h.dynamicTextureMu.RLock()
	data, ok := h.dynamicTextureData[render.TextureID(id)]
	h.dynamicTextureMu.RUnlock()
	if !ok {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(data)
}

func initDynamicTextures(manifest asset.Manifest) map[render.TextureID][]byte {
	result := make(map[render.TextureID][]byte)
	for _, d := range manifest.Textures() {
		if !d.Dynamic {
			continue
		}
		img := image.NewRGBA(image.Rect(0, 0, d.W, d.H))
		var buf bytes.Buffer
		if err := png.Encode(&buf, img); err != nil {
			continue
		}
		result[d.ID] = buf.Bytes()
	}
	return result
}
