import { render } from "@testing-library/react";
import { StrictMode, createElement, useEffect } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { FakeWebSocket } from "../../test-utils/ws";
import type * as SocketModule from "./socket";

const KEEPALIVE_FRAME = JSON.stringify({ type: "ping", data: {} });

let socket: typeof SocketModule;

function sockets(): FakeWebSocket[] {
    return FakeWebSocket.instances;
}

function liveSockets(): FakeWebSocket[] {
    return FakeWebSocket.instances.filter(instance => instance.readyState === FakeWebSocket.OPEN);
}

function lastSocket(): FakeWebSocket {
    const instance = FakeWebSocket.instances[FakeWebSocket.instances.length - 1];
    if (!instance) {
        throw new Error("no websocket was opened");
    }

    return instance;
}

function openSocket(instance: FakeWebSocket = lastSocket()): void {
    instance.onopen?.();
}

function emit(msg: { type: string; data?: unknown }, instance: FakeWebSocket = lastSocket()): void {
    instance.onmessage?.({ data: JSON.stringify(msg) });
}

function setVisibility(state: "visible" | "hidden"): void {
    Object.defineProperty(document, "visibilityState", { configurable: true, get: () => state });
}

let effectRuns = 0;

function Subscriber({ sessionKey }: { sessionKey: string }) {
    useEffect(() => {
        effectRuns += 1;
        socket.openRealtimeSocket(sessionKey);

        return () => {
            socket.closeRealtimeSocket(sessionKey);
        };
    }, [sessionKey]);

    return null;
}

function renderInStrictMode(sessionKey: string) {
    return render(createElement(StrictMode, null, createElement(Subscriber, { sessionKey })));
}

beforeEach(async () => {
    FakeWebSocket.instances = [];
    effectRuns = 0;
    vi.stubGlobal("WebSocket", FakeWebSocket);
    vi.resetModules();
    socket = await import("./socket");
});

afterEach(() => {
    vi.restoreAllMocks();
    Reflect.deleteProperty(document, "visibilityState");
});

describe("realtime socket refcount", () => {
    it("opens a single socket for the first holder", () => {
        // when
        socket.openRealtimeSocket("user:u1");

        // then
        expect(sockets()).toHaveLength(1);
        expect(liveSockets()).toHaveLength(1);
    });

    it("leaves exactly one live socket after an open, a close and an open", () => {
        // given
        socket.openRealtimeSocket("user:u1");

        // when
        socket.closeRealtimeSocket("user:u1");
        socket.openRealtimeSocket("user:u1");

        // then
        expect(liveSockets()).toHaveLength(1);
        expect(lastSocket().readyState).toBe(FakeWebSocket.OPEN);
    });

    it("keeps the socket open while a second holder still has it", () => {
        // given
        socket.openRealtimeSocket("user:u1");
        socket.openRealtimeSocket("user:u1");
        const opened = lastSocket();

        // when
        socket.closeRealtimeSocket("user:u1");

        // then
        expect(sockets()).toHaveLength(1);
        expect(opened.readyState).toBe(FakeWebSocket.OPEN);
        expect(opened.closeCount).toBe(0);
    });

    it("closes the socket once every holder has let go", () => {
        // given
        socket.openRealtimeSocket("user:u1");
        socket.openRealtimeSocket("user:u1");
        const opened = lastSocket();

        // when
        socket.closeRealtimeSocket("user:u1");
        socket.closeRealtimeSocket("user:u1");

        // then
        expect(opened.readyState).toBe(FakeWebSocket.CLOSED);
        expect(opened.closeCount).toBe(1);
        expect(liveSockets()).toHaveLength(0);
    });

    it("reuses the live socket instead of opening a second one for another holder", () => {
        // given
        socket.openRealtimeSocket("user:u1");

        // when
        socket.openRealtimeSocket("user:u1");

        // then
        expect(sockets()).toHaveLength(1);
    });

    it("ignores a close that never had a matching open", () => {
        // when
        const close = () => socket.closeRealtimeSocket("user:u1");

        // then
        expect(close).not.toThrow();
        expect(sockets()).toHaveLength(0);
    });

    it("does not let unmatched closes push the refcount below zero", () => {
        // given
        socket.closeRealtimeSocket("user:u1");
        socket.closeRealtimeSocket("user:u1");
        socket.closeRealtimeSocket("user:u1");

        // when
        socket.openRealtimeSocket("user:u1");
        socket.closeRealtimeSocket("user:u1");

        // then
        expect(sockets()).toHaveLength(1);
        expect(lastSocket().readyState).toBe(FakeWebSocket.CLOSED);
    });

    it("opens a brand new socket after a full close rather than reviving the dead one", () => {
        // given
        socket.openRealtimeSocket("user:u1");
        const first = lastSocket();
        socket.closeRealtimeSocket("user:u1");

        // when
        socket.openRealtimeSocket("user:u1");
        const second = lastSocket();

        // then
        expect(sockets()).toHaveLength(2);
        expect(second).not.toBe(first);
        expect(first.readyState).toBe(FakeWebSocket.CLOSED);
        expect(second.readyState).toBe(FakeWebSocket.OPEN);
    });
});

describe("realtime socket session key", () => {
    it("replaces the socket when the session key changes", () => {
        // given
        socket.openRealtimeSocket("user:u1");
        const first = lastSocket();

        // when
        socket.openRealtimeSocket("user:u2");

        // then
        expect(sockets()).toHaveLength(2);
        expect(first.readyState).toBe(FakeWebSocket.CLOSED);
        expect(liveSockets()).toEqual([lastSocket()]);
    });

    it("replaces the socket when a guest signs in, even though only the kind changed", () => {
        // given
        socket.openRealtimeSocket("guest:g1");
        const guest = lastSocket();

        // when
        socket.openRealtimeSocket("user:g1");

        // then
        expect(guest.readyState).toBe(FakeWebSocket.CLOSED);
        expect(liveSockets()).toEqual([lastSocket()]);
    });

    it("ignores a stale close from a session key that has already been replaced", () => {
        // given
        socket.openRealtimeSocket("user:u1");
        socket.openRealtimeSocket("user:u2");
        const second = lastSocket();

        // when
        socket.closeRealtimeSocket("user:u1");

        // then
        expect(second.readyState).toBe(FakeWebSocket.OPEN);
        expect(liveSockets()).toEqual([second]);
    });

    it("closes the new socket when its own holder lets go", () => {
        // given
        socket.openRealtimeSocket("user:u1");
        socket.openRealtimeSocket("user:u2");
        const second = lastSocket();

        // when
        socket.closeRealtimeSocket("user:u1");
        socket.closeRealtimeSocket("user:u2");

        // then
        expect(second.readyState).toBe(FakeWebSocket.CLOSED);
        expect(liveSockets()).toHaveLength(0);
    });
});

describe("realtime socket under strict mode", () => {
    it("really does mount, unmount and mount again", () => {
        // when
        renderInStrictMode("user:u1");

        // then
        expect(effectRuns).toBe(2);
    });

    it("still holds a live socket after the double mount", () => {
        // when
        renderInStrictMode("user:u1");

        // then
        expect(effectRuns).toBe(2);
        expect(liveSockets()).toHaveLength(1);
        expect(lastSocket().readyState).toBe(FakeWebSocket.OPEN);
    });

    it("closes every socket it opened once the tree unmounts", () => {
        // given
        const view = renderInStrictMode("user:u1");

        // when
        view.unmount();

        // then
        expect(liveSockets()).toHaveLength(0);
        for (const instance of sockets()) {
            expect(instance.readyState).toBe(FakeWebSocket.CLOSED);
        }
    });

    it("holds a live socket for the new user after a signed in user changes", () => {
        // given
        const view = renderInStrictMode("user:u1");
        const before = sockets().length;

        // when
        view.rerender(createElement(StrictMode, null, createElement(Subscriber, { sessionKey: "user:u2" })));

        // then
        expect(sockets().length).toBeGreaterThan(before);
        expect(liveSockets()).toEqual([lastSocket()]);
    });
});

describe("realtime socket url", () => {
    it("connects to the socket route on the page's own origin, with no query string", () => {
        // when
        socket.openRealtimeSocket("user:u1");

        // then
        expect(sockets()).toHaveLength(1);
        expect(lastSocket().url).toBe("ws://localhost:3000/api/v1/ws");
    });

    it("uses the same url for a guest, because the cookie says who is connecting", () => {
        // when
        socket.openRealtimeSocket("guest:g1");

        // then
        expect(lastSocket().url).toBe("ws://localhost:3000/api/v1/ws");
    });
});

describe("realtime socket send", () => {
    it("sends a message down an open socket", () => {
        // given
        socket.openRealtimeSocket("user:u1");
        openSocket();
        const frame = JSON.stringify({ type: "ping", data: { nonce: 1 } });

        // when
        socket.sendRaw(frame);

        // then
        expect(lastSocket().sent).toEqual([frame]);
    });

    it("drops a message when the socket is not open", () => {
        // given
        socket.openRealtimeSocket("user:u1");
        openSocket();
        const opened = lastSocket();
        opened.readyState = FakeWebSocket.CLOSED;

        // when
        socket.sendRaw(JSON.stringify({ type: "ping", data: { nonce: 1 } }));

        // then
        expect(opened.sent).toHaveLength(0);
    });

    it("drops a message when no socket has been opened at all", () => {
        // when
        const send = () => socket.sendRaw(JSON.stringify({ type: "ping", data: { nonce: 1 } }));

        // then
        expect(send).not.toThrow();
        expect(sockets()).toHaveLength(0);
    });

    it("drops a message once every holder has let go", () => {
        // given
        socket.openRealtimeSocket("user:u1");
        openSocket();
        const opened = lastSocket();
        socket.closeRealtimeSocket("user:u1");

        // when
        socket.sendRaw(JSON.stringify({ type: "ping", data: { nonce: 1 } }));

        // then
        expect(opened.sent).toHaveLength(0);
    });
});

describe("realtime socket frames and status", () => {
    it("hands every frame to each frame handler as the raw text that arrived", () => {
        // given
        const first = vi.fn();
        const second = vi.fn();
        socket.onFrame(first);
        socket.onFrame(second);
        socket.openRealtimeSocket("user:u1");
        openSocket();

        // when
        emit({ type: "pong", data: { nonce: 1 } });

        // then
        expect(first).toHaveBeenCalledWith('{"type":"pong","data":{"nonce":1}}');
        expect(second).toHaveBeenCalledWith('{"type":"pong","data":{"nonce":1}}');
    });

    it("stops handing frames to a frame handler that has let go", () => {
        // given
        const handler = vi.fn();
        const release = socket.onFrame(handler);
        socket.openRealtimeSocket("user:u1");
        openSocket();

        // when
        release();
        emit({ type: "pong", data: {} });

        // then
        expect(handler).not.toHaveBeenCalled();
    });

    it("tells each status handler once per open, never per frame", () => {
        // given
        const handler = vi.fn();
        socket.onStatus(handler);
        socket.openRealtimeSocket("user:u1");

        // when
        openSocket();
        emit({ type: "pong", data: {} });
        emit({ type: "pong", data: {} });

        // then
        expect(handler).toHaveBeenCalledExactlyOnceWith(true);
    });

    it("tells each status handler when the open socket closes", () => {
        // given
        vi.useFakeTimers();
        const handler = vi.fn();
        socket.onStatus(handler);
        socket.openRealtimeSocket("user:u1");
        openSocket();

        // when
        lastSocket().onclose?.();

        // then
        expect(handler.mock.calls).toEqual([[true], [false]]);
    });

    it("stops telling a status handler that has let go", () => {
        // given
        const handler = vi.fn();
        const release = socket.onStatus(handler);
        socket.openRealtimeSocket("user:u1");

        // when
        release();
        openSocket();

        // then
        expect(handler).not.toHaveBeenCalled();
    });
});

describe("realtime socket keepalive", () => {
    it("pings the server while the connection is idle", () => {
        // given
        vi.useFakeTimers();
        vi.setSystemTime(new Date("2026-08-02T12:00:00Z"));
        socket.openRealtimeSocket("user:u1");
        openSocket();

        // when
        vi.advanceTimersByTime(20_000);

        // then
        expect(lastSocket().sent).toEqual([KEEPALIVE_FRAME]);
    });

    it("probes a connection that has gone quiet for too long rather than closing it", () => {
        // given
        vi.useFakeTimers();
        vi.setSystemTime(new Date("2026-08-02T12:00:00Z"));
        socket.openRealtimeSocket("user:u1");
        openSocket();
        const opened = lastSocket();

        // when
        vi.advanceTimersByTime(100_000);

        // then
        expect(opened.sent).toHaveLength(5);
        expect(opened.closeCount).toBe(0);
    });

    it("closes a quiet connection once the probe goes unanswered", () => {
        // given
        vi.useFakeTimers();
        vi.setSystemTime(new Date("2026-08-02T12:00:00Z"));
        socket.openRealtimeSocket("user:u1");
        openSocket();
        const opened = lastSocket();

        // when
        vi.advanceTimersByTime(100_000);
        vi.advanceTimersByTime(5_000);

        // then
        expect(opened.closeCount).toBe(1);
    });

    it("keeps a quiet connection that answers the probe", () => {
        // given
        vi.useFakeTimers();
        vi.setSystemTime(new Date("2026-08-02T12:00:00Z"));
        socket.openRealtimeSocket("user:u1");
        openSocket();
        const opened = lastSocket();

        // when
        vi.advanceTimersByTime(100_000);
        vi.advanceTimersByTime(1_000);
        emit({ type: "pong", data: {} }, opened);
        vi.advanceTimersByTime(10_000);

        // then
        expect(opened.closeCount).toBe(0);
    });

    it("keeps the connection alive while messages are still arriving", () => {
        // given
        vi.useFakeTimers();
        vi.setSystemTime(new Date("2026-08-02T12:00:00Z"));
        socket.openRealtimeSocket("user:u1");
        openSocket();
        const opened = lastSocket();

        // when
        vi.advanceTimersByTime(80_000);
        emit({ type: "pong", data: { nonce: 2 } }, opened);
        vi.advanceTimersByTime(40_000);

        // then
        expect(opened.closeCount).toBe(0);
    });

    it("counts a frame the parser would reject as a sign of life, because liveness is judged before parsing", () => {
        // given
        vi.useFakeTimers();
        vi.setSystemTime(new Date("2026-08-02T12:00:00Z"));
        socket.openRealtimeSocket("user:u1");
        openSocket();
        const opened = lastSocket();

        // when
        vi.advanceTimersByTime(100_000);
        vi.advanceTimersByTime(1_000);
        opened.onmessage?.({ data: "not json at all" });
        vi.advanceTimersByTime(10_000);

        // then
        expect(opened.closeCount).toBe(0);
    });
});

describe("realtime socket reconnect", () => {
    it("reconnects with a growing backoff after the connection drops", () => {
        // given
        vi.useFakeTimers();
        vi.spyOn(Math, "random").mockReturnValue(1);
        socket.openRealtimeSocket("user:u1");
        openSocket();

        // when
        lastSocket().onclose?.();

        // then
        expect(sockets()).toHaveLength(1);
        vi.advanceTimersByTime(1000);
        expect(sockets()).toHaveLength(2);

        lastSocket().onclose?.();
        vi.advanceTimersByTime(1000);
        expect(sockets()).toHaveLength(2);
        vi.advanceTimersByTime(1000);
        expect(sockets()).toHaveLength(3);
    });

    it("never lets the backoff grow past thirty seconds", () => {
        // given
        vi.useFakeTimers();
        vi.spyOn(Math, "random").mockReturnValue(1);
        socket.openRealtimeSocket("user:u1");
        openSocket();
        for (const wait of [1_000, 2_000, 4_000, 8_000, 16_000, 30_000]) {
            lastSocket().onclose?.();
            vi.advanceTimersByTime(wait);
        }
        const before = sockets().length;

        // when
        lastSocket().onclose?.();
        vi.advanceTimersByTime(30_000);

        // then
        expect(sockets()).toHaveLength(before + 1);
    });

    it("starts the backoff again from one second once a connection opens", () => {
        // given
        vi.useFakeTimers();
        vi.spyOn(Math, "random").mockReturnValue(1);
        socket.openRealtimeSocket("user:u1");
        openSocket();
        lastSocket().onclose?.();
        vi.advanceTimersByTime(1_000);
        lastSocket().onclose?.();
        vi.advanceTimersByTime(2_000);
        openSocket();
        const before = sockets().length;

        // when
        lastSocket().onclose?.();
        vi.advanceTimersByTime(1_000);

        // then
        expect(sockets()).toHaveLength(before + 1);
    });

    it("spreads the reconnect across the backoff window instead of retrying in lockstep", () => {
        // given
        vi.useFakeTimers();
        vi.spyOn(Math, "random").mockReturnValue(0);
        socket.openRealtimeSocket("user:u1");
        openSocket();

        // when
        lastSocket().onclose?.();
        vi.advanceTimersByTime(0);

        // then
        expect(sockets()).toHaveLength(2);
    });

    it("never waits longer than the backoff cap", () => {
        // given
        vi.useFakeTimers();
        vi.spyOn(Math, "random").mockReturnValue(0.999);
        socket.openRealtimeSocket("user:u1");
        openSocket();

        // when
        lastSocket().onclose?.();
        vi.advanceTimersByTime(1000);

        // then
        expect(sockets()).toHaveLength(2);
    });

    it("ignores a late close from a socket that has already been replaced", () => {
        // given
        vi.useFakeTimers();
        socket.openRealtimeSocket("user:u1");
        openSocket();
        const stale = lastSocket();
        socket.openRealtimeSocket("user:u2");
        const live = lastSocket();
        openSocket(live);
        const frame = JSON.stringify({ type: "ping", data: { nonce: 1 } });

        // when
        stale.onclose?.();
        vi.advanceTimersByTime(5000);

        // then
        expect(sockets()).toHaveLength(2);
        socket.sendRaw(frame);
        expect(live.sent).toContain(frame);
    });

    it("never reconnects once the provider has gone away", () => {
        // given
        vi.useFakeTimers();
        socket.openRealtimeSocket("user:u1");
        openSocket();
        const opened = lastSocket();

        // when
        socket.closeRealtimeSocket("user:u1");
        opened.onclose?.();
        vi.advanceTimersByTime(30_000);

        // then
        expect(sockets()).toHaveLength(1);
    });

    it("cancels a pending reconnect when the provider goes away during the wait", () => {
        // given
        vi.useFakeTimers();
        vi.spyOn(Math, "random").mockReturnValue(1);
        socket.openRealtimeSocket("user:u1");
        openSocket();
        lastSocket().onclose?.();

        // when
        socket.closeRealtimeSocket("user:u1");
        vi.advanceTimersByTime(30_000);

        // then
        expect(sockets()).toHaveLength(1);
    });

    it("closes a socket that errored so the reconnect can take over", () => {
        // given
        socket.openRealtimeSocket("user:u1");
        openSocket();
        const opened = lastSocket();

        // when
        opened.onerror?.();

        // then
        expect(opened.closeCount).toBe(1);
    });
});

describe("realtime socket visibility", () => {
    it("pings the server when the tab becomes visible again", () => {
        // given
        setVisibility("visible");
        socket.openRealtimeSocket("user:u1");
        openSocket();

        // when
        document.dispatchEvent(new Event("visibilitychange"));

        // then
        expect(lastSocket().sent).toEqual([KEEPALIVE_FRAME]);
    });

    it("probes a stale socket when the tab becomes visible again rather than closing it", () => {
        // given
        vi.useFakeTimers();
        setVisibility("visible");
        socket.openRealtimeSocket("user:u1");
        const opened = lastSocket();

        // when
        document.dispatchEvent(new Event("visibilitychange"));

        // then
        expect(opened.sent).toHaveLength(1);
        expect(opened.closeCount).toBe(0);
    });

    it("closes the socket when the visibility probe goes unanswered", () => {
        // given
        vi.useFakeTimers();
        setVisibility("visible");
        socket.openRealtimeSocket("user:u1");
        const opened = lastSocket();

        // when
        document.dispatchEvent(new Event("visibilitychange"));
        vi.advanceTimersByTime(5_000);

        // then
        expect(opened.closeCount).toBe(1);
    });

    it("keeps the socket when the visibility probe is answered", () => {
        // given
        vi.useFakeTimers();
        setVisibility("visible");
        socket.openRealtimeSocket("user:u1");
        const opened = lastSocket();

        // when
        document.dispatchEvent(new Event("visibilitychange"));
        vi.advanceTimersByTime(1_000);
        emit({ type: "pong", data: {} }, opened);
        vi.advanceTimersByTime(10_000);

        // then
        expect(opened.closeCount).toBe(0);
    });

    it("sends one probe however often the tab turns visible while it is waiting", () => {
        // given
        vi.useFakeTimers();
        setVisibility("visible");
        socket.openRealtimeSocket("user:u1");
        const opened = lastSocket();

        // when
        document.dispatchEvent(new Event("visibilitychange"));
        document.dispatchEvent(new Event("visibilitychange"));
        document.dispatchEvent(new Event("visibilitychange"));

        // then
        expect(opened.sent).toEqual([KEEPALIVE_FRAME]);
    });

    it("does nothing when the tab is being hidden", () => {
        // given
        setVisibility("hidden");
        socket.openRealtimeSocket("user:u1");
        openSocket();
        const opened = lastSocket();

        // when
        document.dispatchEvent(new Event("visibilitychange"));

        // then
        expect(opened.sent).toHaveLength(0);
        expect(opened.closeCount).toBe(0);
    });

    it("listens for visibility once, however many holders share the socket", () => {
        // given
        const addListener = vi.spyOn(document, "addEventListener");

        // when
        socket.openRealtimeSocket("user:u1");
        socket.openRealtimeSocket("user:u1");

        // then
        expect(addListener.mock.calls.filter(([type]) => type === "visibilitychange")).toHaveLength(1);
    });

    it("stops listening for visibility once every holder has let go", () => {
        // given
        const removeListener = vi.spyOn(document, "removeEventListener");
        socket.openRealtimeSocket("user:u1");

        // when
        socket.closeRealtimeSocket("user:u1");

        // then
        expect(removeListener).toHaveBeenCalledWith("visibilitychange", expect.any(Function));
    });
});

describe("realtime socket when the network comes back", () => {
    function openWatchingTheNetwork(): () => void {
        const addListener = vi.spyOn(window, "addEventListener");
        socket.openRealtimeSocket("user:u1");

        const call = addListener.mock.calls.find(([type]) => type === "online");
        if (!call) {
            throw new Error("the socket never listened for the network coming back");
        }

        return call[1] as () => void;
    }

    function dropRepeatedly(): void {
        for (const wait of [1_000, 2_000, 4_000]) {
            lastSocket().onclose?.();
            vi.advanceTimersByTime(wait);
        }

        lastSocket().onclose?.();
    }

    it("reconnects at once instead of sitting out the rest of its backoff", () => {
        // given
        vi.useFakeTimers();
        vi.spyOn(Math, "random").mockReturnValue(1);
        const comeBackOnline = openWatchingTheNetwork();
        openSocket();
        dropRepeatedly();
        const before = sockets().length;

        // when
        comeBackOnline();

        // then
        expect(sockets()).toHaveLength(before + 1);
        vi.advanceTimersByTime(30_000);
        expect(sockets()).toHaveLength(before + 1);
    });

    it("starts the backoff again from one second after the network comes back", () => {
        // given
        vi.useFakeTimers();
        vi.spyOn(Math, "random").mockReturnValue(1);
        const comeBackOnline = openWatchingTheNetwork();
        openSocket();
        dropRepeatedly();
        comeBackOnline();
        const before = sockets().length;

        // when
        lastSocket().onclose?.();
        vi.advanceTimersByTime(1_000);

        // then
        expect(sockets()).toHaveLength(before + 1);
    });

    it("probes a socket it still holds rather than replacing it", () => {
        // given
        vi.useFakeTimers();
        const comeBackOnline = openWatchingTheNetwork();
        openSocket();
        const opened = lastSocket();

        // when
        comeBackOnline();

        // then
        expect(sockets()).toHaveLength(1);
        expect(opened.sent).toEqual([KEEPALIVE_FRAME]);
    });

    it("stops listening for the network once every holder has let go", () => {
        // given
        const comeBackOnline = openWatchingTheNetwork();
        const removeListener = vi.spyOn(window, "removeEventListener");

        // when
        socket.closeRealtimeSocket("user:u1");

        // then
        expect(removeListener).toHaveBeenCalledWith("online", comeBackOnline);
    });
});
