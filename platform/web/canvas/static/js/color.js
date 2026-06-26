export function colorToCSS(color) {
    if (!color) {
        return "rgba(0, 0, 0, 0)";
    }

    const r = Math.round(clamp01(color.r) * 255);
    const g = Math.round(clamp01(color.g) * 255);
    const b = Math.round(clamp01(color.b) * 255);
    const a = clamp01(color.a);

    return `rgba(${r}, ${g}, ${b}, ${a})`;
}

export function hasAlpha(color) {
    return color && color.a > 0;
}

function clamp01(value) {
    return Math.min(1, Math.max(0, value || 0));
}
