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
	TextureID uint32 `json:"textureID"`
	Version   int    `json:"version"`
}

type textureUpdateListDTO struct {
	Type    string             `json:"type"`
	Updates []textureUpdateDTO `json:"updates"`
}

// handleTextureUpdates re-encodes changed dynamic textures to PNG and bumps
// their version. It returns a DTO entry per texture that actually changed, so
// the caller can queue a "textureUpdate" WS message for clients.
func (h *hub) handleTextureUpdates(updates []runtime.TextureUpdate) []textureUpdateDTO {
	if len(updates) == 0 {
		return nil
	}
	h.dynamicTextureMu.Lock()
	defer h.dynamicTextureMu.Unlock()
	changed := make([]textureUpdateDTO, 0, len(updates))
	for _, u := range updates {
		if u.Image == nil {
			continue
		}
		var buf bytes.Buffer
		if err := png.Encode(&buf, u.Image); err != nil {
			continue
		}
		h.dynamicTextureData[u.ID] = buf.Bytes()
		h.textureVersion[u.ID]++
		changed = append(changed, textureUpdateDTO{TextureID: uint32(u.ID), Version: h.textureVersion[u.ID]})
	}
	return changed
}

// enqueueTextureUpdates applies driver-produced texture updates and queues a
// WS notification for connected clients, mirroring enqueueAudio.
func (h *hub) enqueueTextureUpdates(updates []runtime.TextureUpdate) {
	changed := h.handleTextureUpdates(updates)
	if len(changed) == 0 {
		return
	}
	h.textureVersionMu.Lock()
	defer h.textureVersionMu.Unlock()
	h.pendingTexture = append(h.pendingTexture, changed...)
}

func (h *hub) drainTextureUpdates() []textureUpdateDTO {
	h.textureVersionMu.Lock()
	defer h.textureVersionMu.Unlock()
	out := append([]textureUpdateDTO(nil), h.pendingTexture...)
	h.pendingTexture = nil
	return out
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
