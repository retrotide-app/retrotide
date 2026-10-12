import { socketUrl } from "../origin";
import type { RealtimeCommand } from "./outbound";

type FrameHandler = (raw: string) => void;

type StatusHandler = (open: boolean) => void;

type SocketState = {
    sessionKey: string | null;
    refCount: number;
    socket: WebSocket | null;
    backoff: number;
    reconnectTimer: ReturnType<typeof setTimeout> | null;
    keepaliveTimer: ReturnType<typeof setInterval> | null;
    probeTimer: ReturnType<typeof setTimeout> | null;
    lastMessageAt: number;
    visibilityBound: boolean;
};

const INITIAL_BACKOFF_MS = 1_000;
const MAX_BACKOFF_MS = 30_000;
const KEEPALIVE_INTERVAL_MS = 20_000;
const STALE_THRESHOLD_MS = 90_000;
const PROBE_TIMEOUT_MS = 5_000;

const KEEPALIVE_FRAME = JSON.stringify({ type: "ping", data: {} } satisfies RealtimeCommand);

const state: SocketState = {
    sessionKey: null,
    refCount: 0,
    socket: null,
    backoff: INITIAL_BACKOFF_MS,
    reconnectTimer: null,
    keepaliveTimer: null,
    probeTimer: null,
    lastMessageAt: 0,
    visibilityBound: false,
};

const frameHandlers = new Set<FrameHandler>();
const statusHandlers = new Set<StatusHandler>();

function notifyStatus(open: boolean): void {
    for (const handler of statusHandlers) {
        handler(open);
    }
}

function clearReconnectTimer(): void {
    if (state.reconnectTimer !== null) {
        clearTimeout(state.reconnectTimer);
        state.reconnectTimer = null;
    }
}

function clearKeepaliveTimer(): void {
    if (state.keepaliveTimer !== null) {
        clearInterval(state.keepaliveTimer);
        state.keepaliveTimer = null;
    }

    if (state.probeTimer !== null) {
        clearTimeout(state.probeTimer);
        state.probeTimer = null;
    }
}

function probeSocket(socket: WebSocket): void {
    if (socket.readyState !== WebSocket.OPEN) {
        return;
    }

    if (state.probeTimer !== null) {
        return;
    }

    const probedAt = Date.now();
    socket.send(KEEPALIVE_FRAME);

    state.probeTimer = setTimeout(() => {
        state.probeTimer = null;

        if (state.socket !== socket) {
            return;
        }

        if (state.lastMessageAt < probedAt) {
            socket.close();
        }
    }, PROBE_TIMEOUT_MS);
}

function closeSocket(): void {
    clearReconnectTimer();
    clearKeepaliveTimer();

    if (state.socket) {
        state.socket.close();
        state.socket = null;
        notifyStatus(false);
    }
}

function onVisible(): void {
    if (document.visibilityState !== "visible") {
        return;
    }

    const socket = state.socket;
    if (!socket) {
        return;
    }

    probeSocket(socket);
}

function onOnline(): void {
    const socket = state.socket;
    if (socket) {
        probeSocket(socket);
        return;
    }

    if (state.reconnectTimer === null) {
        return;
    }

    state.backoff = INITIAL_BACKOFF_MS;
    connect();
}

function bindBrowserSignals(): void {
    if (state.visibilityBound) {
        return;
    }

    document.addEventListener("visibilitychange", onVisible);
    window.addEventListener("online", onOnline);
    state.visibilityBound = true;
}

function unbindBrowserSignals(): void {
    if (!state.visibilityBound) {
        return;
    }

    document.removeEventListener("visibilitychange", onVisible);
    window.removeEventListener("online", onOnline);
    state.visibilityBound = false;
}

function connect(): void {
    closeSocket();

    const socket = new WebSocket(socketUrl(window.location.origin));
    state.socket = socket;

    socket.onopen = () => {
        state.backoff = INITIAL_BACKOFF_MS;
        state.lastMessageAt = Date.now();

        notifyStatus(true);

        clearKeepaliveTimer();
        state.keepaliveTimer = setInterval(() => {
            if (state.socket !== socket) {
                return;
            }

            if (!document.hidden && Date.now() - state.lastMessageAt > STALE_THRESHOLD_MS) {
                probeSocket(socket);
                return;
            }

            if (socket.readyState === WebSocket.OPEN) {
                socket.send(KEEPALIVE_FRAME);
            }
        }, KEEPALIVE_INTERVAL_MS);
    };

    socket.onmessage = event => {
        state.lastMessageAt = Date.now();

        const raw = event.data as string;

        for (const handler of frameHandlers) {
            handler(raw);
        }
    };

    socket.onclose = () => {
        if (state.socket !== socket) {
            return;
        }

        state.socket = null;
        clearKeepaliveTimer();
        notifyStatus(false);

        if (state.refCount === 0) {
            return;
        }

        const cap = Math.min(state.backoff, MAX_BACKOFF_MS);
        state.backoff = Math.min(cap * 2, MAX_BACKOFF_MS);

        const delay = Math.random() * cap;

        state.reconnectTimer = setTimeout(() => {
            connect();
        }, delay);
    };

    socket.onerror = () => {
        socket.close();
    };
}

export function openRealtimeSocket(sessionKey: string): void {
    if (state.sessionKey !== sessionKey) {
        closeSocket();
        state.sessionKey = sessionKey;
        state.refCount = 0;
    }

    state.refCount += 1;
    bindBrowserSignals();

    if (state.socket === null) {
        connect();
    }
}

export function closeRealtimeSocket(sessionKey: string): void {
    if (state.sessionKey !== sessionKey) {
        return;
    }

    state.refCount -= 1;

    if (state.refCount > 0) {
        return;
    }

    state.refCount = 0;
    state.sessionKey = null;

    unbindBrowserSignals();
    closeSocket();

    state.backoff = INITIAL_BACKOFF_MS;
    state.lastMessageAt = 0;
}

export function sendRaw(payload: string): void {
    const socket = state.socket;
    if (socket === null || socket.readyState !== WebSocket.OPEN) {
        return;
    }

    socket.send(payload);
}

export function onFrame(handler: FrameHandler): () => void {
    frameHandlers.add(handler);

    return () => {
        frameHandlers.delete(handler);
    };
}

export function onStatus(handler: StatusHandler): () => void {
    statusHandlers.add(handler);

    return () => {
        statusHandlers.delete(handler);
    };
}
