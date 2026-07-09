import { colorToCSS, hasAlpha } from "./color.js";
import { applyCamera, resetTransform } from "./transform.js";
import {
    drawRect,
    drawCircle,
    drawPoint,
    drawLine,
    drawArc,
    drawText,
    drawPolygon,
    drawImage,
} from "./primitives.js";

export function createCanvasRenderer(canvas, options = {}) {
    const ctx = canvas.getContext("2d");
    const onResize = options.onResize ?? (() => {});
    const assets = options.assets ?? new Map()
    let latestFrame = null;
    let resizeFrame = 0;

    function resize() {
        resizeFrame = 0;

        const ratio = window.devicePixelRatio || 1;
        const container = canvas.parentElement ?? canvas;
        const rect = container.getBoundingClientRect();
        const width = Math.max(1, Math.floor(rect.width * ratio));
        const height = Math.max(1, Math.floor(rect.height * ratio));

        if (canvas.width === width && canvas.height === height) {
            return;
        }

        canvas.width = width;
        canvas.height = height;
        onResize(size());

        if (latestFrame) {
            render(latestFrame);
        }
    }

    function render(frame) {
        latestFrame = frame;

        resetTransform(ctx);
        ctx.clearRect(0, 0, canvas.width, canvas.height);

        if (hasAlpha(frame.clear)) {
            ctx.fillStyle = colorToCSS(frame.clear);
            ctx.fillRect(0, 0, canvas.width, canvas.height);
        }

        applyCamera(ctx, frame.camera);

        for (const command of frame.commands) {
            drawCommand(ctx, command);
        }

        resetTransform(ctx);
    }

    function setAssets(map) {
        for (const [key, value] of map) {
            assets.set(key, value)
        }
    }

    function drawCommand(ctx, command) {
        const clipped = command.clip && command.clip.width > 0 && command.clip.height > 0;
        if (clipped) {
            ctx.save();
            ctx.beginPath();
            ctx.rect(command.clip.x, command.clip.y, command.clip.width, command.clip.height);
            ctx.clip();
        }

        switch (command.type) {
            case "rect":
                drawRect(ctx, command);
                break;
            case "circle":
                drawCircle(ctx, command);
                break;
            case "point":
                drawPoint(ctx, command);
                break;
            case "line":
                drawLine(ctx, command);
                break;
            case "arc":
                drawArc(ctx, command);
                break;
            case "text":
                drawText(ctx, command);
                break;
            case "polygon":
                drawPolygon(ctx, command);
                break;
            case "image":
                drawImage(ctx, command, assets);
                break;
        }

        if (clipped) {
            ctx.restore();
        }
    }

    function scheduleResize() {
        if (resizeFrame !== 0) {
            return;
        }

        resizeFrame = requestAnimationFrame(resize);
    }

    const resizeObserver = new ResizeObserver(scheduleResize);
    resizeObserver.observe(canvas.parentElement ?? canvas);
    window.addEventListener("resize", scheduleResize);
    resize();

    return {
        render,
        resize,
        destroy,
        size,
        setAssets,
    };

    function destroy() {
        resizeObserver.disconnect();
        window.removeEventListener("resize", scheduleResize);
        if (resizeFrame !== 0) {
            cancelAnimationFrame(resizeFrame);
            resizeFrame = 0;
        }
    }

    function size() {
        return {
            width: canvas.width,
            height: canvas.height,
        };
    }
}
