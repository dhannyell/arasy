import { connectFrameSocket } from "./socket.js";
import { createCanvasRenderer } from "./renderer.js";

const canvas = document.getElementById("ff-canvas");
const stats = document.getElementById("stats");
const historyStats = document.getElementById("history");
const playPauseButton = document.getElementById("play-pause");
const backButton = document.getElementById("back");
const stepButton = document.getElementById("step");
const resetButton = document.getElementById("reset");
const speedInput = document.getElementById("speed");
const speedValue = document.getElementById("speed-value");
const debugInput = document.getElementById("debug");

let connection = null;
let paused = false;

const renderer = createCanvasRenderer(canvas, {
    onResize(size) {
        connection?.send({
            type: "resize",
            width: size.width,
            height: size.height,
        });
    },
});

playPauseButton.addEventListener("click", () => {
    paused = !paused;
    playPauseButton.textContent = paused ? "Play" : "Pause";

    connection?.send({
        type: "control",
        action: paused ? "pause" : "play",
    });
});

stepButton.addEventListener("click", () => {
    paused = true;
    playPauseButton.textContent = "Play";

    connection?.send({
        type: "control",
        action: "step",
    });
});

backButton.addEventListener("click", () => {
    paused = true;
    playPauseButton.textContent = "Play";

    connection?.send({
        type: "control",
        action: "back",
    });
});

resetButton.addEventListener("click", () => {
    connection?.send({
        type: "control",
        action: "reset",
    });
});

speedInput.addEventListener("input", () => {
    const speed = Number(speedInput.value);
    speedValue.textContent = `${speed}x`;

    connection?.send({
        type: "control",
        action: "setSpeed",
        speed,
    });
});

debugInput.addEventListener("change", () => {
    connection?.send({
        type: "control",
        action: "setDebug",
        debug: debugInput.checked,
    });
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
        syncControls(frame.state);
        stats.textContent = `tick ${frame.tick} | commands ${frame.commands.length}`;
    },
});

function syncControls(state) {
    if (!state) {
        return;
    }

    paused = state.paused;
    playPauseButton.textContent = paused ? "Play" : "Pause";

    if (Number(speedInput.value) !== state.speed) {
        speedInput.value = String(state.speed);
    }
    speedValue.textContent = `${state.speed}x`;

    debugInput.checked = state.debug;
    historyStats.textContent = `history ${state.historyLen}`;
}
