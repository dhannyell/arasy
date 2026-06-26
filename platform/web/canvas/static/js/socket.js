export function connectFrameSocket(options) {
    const {
        url,
        onOpen = () => {},
        onClose = () => {},
        onError = () => {},
        onFrame = () => {},
        reconnectDelayMs = 750,
    } = options;

    let socket = null;
    let closedByUser = false;

    function connect() {
        const wsURL = resolveWebSocketURL(url);

        socket = new WebSocket(wsURL);

        socket.addEventListener('open', () => {
            onOpen();
        })

        socket.addEventListener('message', (event) => {
            const message = parseMessage(event.data);
            if (!message) {
                return;
            }

            if (message.type === "frame") {
                onFrame(message);
            }
        });

        socket.addEventListener('close', () => {
            socket = null;
            onClose();

            if (!closedByUser) {
                setTimeout(connect, reconnectDelayMs);
            }
        });

        socket.addEventListener('error', (event) => {
            onError(event);

            if (socket) {
                socket.close();
            }
        });
    }

    function send(message) {
        if (!socket || socket.readyState !== WebSocket.OPEN) {
            return false;
        }

        socket.send(JSON.stringify(message));
        return true;
    }

    function close() {
        closedByUser = true;

        if (socket) {
            socket.close();
        }
    }

    connect();

    return {
        send,
        close,
    };
}

function resolveWebSocketURL(url) {
    if (url.startsWith("ws://") || url.startsWith("wss://")) {
        return url;
    }

    const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";

    if (url.startsWith("/")) {
        return `${protocol}//${window.location.host}${url}`;
    }

    return `${protocol}//${window.location.host}/${url}`;
}

function parseMessage(data) {
    try {
        return JSON.parse(data);
    } catch (error) {
        console.error("Invalid websocket message:", error);
        return null;
    }
}
