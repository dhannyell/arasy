// Loads the asset manifest and decodes every texture into an
// ImageBitmap keyed by its per-run TextureID.
export async function loadAssetRegistry() {
    const registry = new Map();
    const res = await fetch("/assets/manifest");
    if (!res.ok) {
        return registry;
    }
    const { textures } = await res.json();
    await Promise.all((textures ?? []).map(async (tex) => {
        try {
            const textureRes = await fetch(tex.url);
            if (!textureRes.ok) {
                return;
            }
            const blob = await textureRes.blob();
            registry.set(tex.id, await createImageBitmap(blob));
        } catch {
            // Missing or invalid textures keep rendering as the placeholder.
        }
    }));
    return registry;
}
