import { renderHook } from "@testing-library/react";
import { useSyncExternalStore } from "react";
import { describe, expect, it, vi } from "vitest";
import { subscribe } from "../api/realtime/bus";
import { FakeWebSocket, emitRealtimeEvent, emitSettledRealtimeEvent, makeWSHarness } from "./ws";

const PONG = { type: "pong", data: { nonce: 1, server_now: "2026-09-22T10:06:02.512Z" } } as const;

describe("FakeWebSocket", () => {
    it("remembers every socket opened and the url it was opened on", () => {
        // given
        FakeWebSocket.instances = [];

        // when
        const socket = new FakeWebSocket("ws://localhost:3000/api/v1/ws");

        // then
        expect(FakeWebSocket.instances).toEqual([socket]);
        expect(socket.url).toBe("ws://localhost:3000/api/v1/ws");
        expect(socket.readyState).toBe(FakeWebSocket.OPEN);
    });

    it("records every frame sent, in order", () => {
        // given
        const socket = new FakeWebSocket("ws://localhost:3000/api/v1/ws");

        // when
        socket.send("first");
        socket.send("second");

        // then
        expect(socket.sent).toEqual(["first", "second"]);
    });

    it("counts each close and reports itself closed", () => {
        // given
        const socket = new FakeWebSocket("ws://localhost:3000/api/v1/ws");

        // when
        socket.close();

        // then
        expect(socket.closeCount).toBe(1);
        expect(socket.readyState).toBe(FakeWebSocket.CLOSED);
    });
});

describe("makeWSHarness delivery", () => {
    it("hands an emitted event to a handler subscribed to that name", () => {
        // given
        const ws = makeWSHarness();
        const handler = vi.fn();
        ws.subscribe("pong", handler);

        // when
        ws.emit(PONG);

        // then
        expect(handler).toHaveBeenCalledWith(PONG);
    });

    it("delivers to a handler that asked for the name inside a list", () => {
        // given
        const ws = makeWSHarness();
        const handler = vi.fn();
        ws.subscribe(["pong"], handler);

        // when
        ws.emit(PONG);
        ws.emit({ type: "pong", data: { nonce: 2 } });

        // then
        expect(handler).toHaveBeenCalledTimes(2);
    });

    it("does not change who receives the current event when a handler unsubscribes during delivery", () => {
        // given
        const ws = makeWSHarness();
        const second = vi.fn();
        const stopSecond = ws.subscribe("pong", () => {
            stopSecond();
        });
        ws.subscribe("pong", second);

        // when
        ws.emit({ type: "pong", data: {} });

        // then
        expect(second).toHaveBeenCalledOnce();
    });
});

describe("makeWSHarness subscription lifetime", () => {
    it("stops delivering once the returned unsubscribe has run", () => {
        // given
        const ws = makeWSHarness();
        const handler = vi.fn();
        const stop = ws.subscribe("pong", handler);

        // when
        stop();
        ws.emit({ type: "pong", data: {} });

        // then
        expect(handler).not.toHaveBeenCalled();
    });

    it("records every unsubscribe on one spy, so a test can assert the consumer let go", () => {
        // given
        const ws = makeWSHarness();
        const stop = ws.subscribe("pong", vi.fn());

        // when
        stop();

        // then
        expect(ws.unsubscribe).toHaveBeenCalledOnce();
    });

    it("keeps a resubscription alive when a stale unsubscribe runs a second time", () => {
        // given
        const ws = makeWSHarness();
        const handler = vi.fn();
        const stop = ws.subscribe("pong", handler);
        stop();
        ws.subscribe("pong", handler);

        // when
        stop();
        ws.emit({ type: "pong", data: {} });

        // then
        expect(handler).toHaveBeenCalledOnce();
    });
});

describe("makeWSHarness reconnects", () => {
    it("starts the epoch at zero and raises it on every reconnect", () => {
        // given
        const ws = makeWSHarness();

        // when
        ws.reconnect();
        ws.reconnect();

        // then
        expect(ws.getEpoch()).toBe(2);
    });

    it("re-renders a consumer reading the epoch through useSyncExternalStore", () => {
        // given
        const ws = makeWSHarness();
        const { result } = renderHook(() => useSyncExternalStore(ws.subscribeEpoch, ws.getEpoch));
        expect(result.current).toBe(0);

        // when
        ws.reconnect();

        // then
        expect(result.current).toBe(1);
    });

    it("stops telling a listener that has gone away", () => {
        // given
        const ws = makeWSHarness();
        const listener = vi.fn();
        const stop = ws.subscribeEpoch(listener);

        // when
        stop();
        ws.reconnect();

        // then
        expect(listener).not.toHaveBeenCalled();
    });
});

describe("makeWSHarness flush", () => {
    it("lets queued work run before it resolves", async () => {
        // given
        const ws = makeWSHarness();
        let settled = false;
        queueMicrotask(() => {
            settled = true;
        });

        // when
        await ws.flush();

        // then
        expect(settled).toBe(true);
    });
});

describe("emitRealtimeEvent", () => {
    it("delivers onto the real bus, for a consumer that may not import it", () => {
        // given
        const handler = vi.fn();
        const stop = subscribe("pong", handler);

        // when
        emitRealtimeEvent(PONG);
        stop();

        // then
        expect(handler).toHaveBeenCalledWith(PONG);
    });

    it("leaves a handler that has let go of the real bus alone", () => {
        // given
        const handler = vi.fn();
        const stop = subscribe("pong", handler);
        stop();

        // when
        emitRealtimeEvent(PONG);

        // then
        expect(handler).not.toHaveBeenCalled();
    });
});

describe("emitSettledRealtimeEvent", () => {
    it("lets the query client's notifications land before it returns", async () => {
        // given
        const handler = vi.fn();
        const stop = subscribe("pong", handler);
        let notified = false;

        // when
        await emitSettledRealtimeEvent(PONG);
        setTimeout(() => {
            notified = true;
        }, 0);
        await emitSettledRealtimeEvent({ type: "pong", data: { nonce: 2 } });
        stop();

        // then
        expect(handler).toHaveBeenCalledTimes(2);
        expect(notified).toBe(true);
    });
});

describe("makeWSHarness outbound", () => {
    it("records the commands a consumer sends", () => {
        // given
        const ws = makeWSHarness();

        // when
        ws.sendRealtime({ type: "ping", data: { nonce: 1 } });

        // then
        expect(ws.sendRealtime).toHaveBeenCalledWith({ type: "ping", data: { nonce: 1 } });
    });
});
