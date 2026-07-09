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
        const blob = await (await fetch(tex.url)).blob();
        registry.set(tex.id, await createImageBitmap(blob));
    }));
    return registry;
}