import { connectFrameSocket } from "./socket.js";
import { createCanvasRenderer } from "./renderer.js";

const canvas = document.getElementById("ff-canvas");
const stats = document.getElementById("stats");

let connection = null;

const renderer = createCanvasRenderer(canvas, {
    onResize(size) {
        connection?.send({
            type: "resize",
            width: size.width,
            height: size.height,
        });
    },
});

connection = connectFrameSocket({
    url: "/ws",

    onOpen() {
        stats.textContent = "connected";
        connection?.send({
            type: "resize",
            ...renderer.size(),
        });
    },

    onClose() {
        stats.textContent = "disconnected, retrying...";
    },

    onError(error) {
        console.error(error);
    },

    onFrame(frame) {
        renderer.render(frame);
        stats.textContent = `tick ${frame.tick} | commands ${frame.commands.length}`;
    },
});
