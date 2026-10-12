import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { FakeWebSocket } from "../../test-utils/ws";
import type * as BusModule from "./bus";
import type * as PipelineModule from "./pipeline";
import type * as SocketModule from "./socket";

const { reportClientError } = vi.hoisted(() => ({ reportClientError: vi.fn() }));

vi.mock("../telemetry", () => ({ reportClientError }));

const SERVER_NOW = "2026-09-22T10:06:02.512Z";

let bus: typeof BusModule;
let pipeline: typeof PipelineModule;
let socket: typeof SocketModule;

function lastSocket(): FakeWebSocket {
    const instance = FakeWebSocket.instances[FakeWebSocket.instances.length - 1];
    if (!instance) {
        throw new Error("no websocket was opened");
    }

    return instance;
}

function connect(): void {
    pipeline.ensureRealtimePipeline();
    socket.openRealtimeSocket("user:u1");
    lastSocket().onopen?.();
}

function receiveRaw(raw: string): void {
    lastSocket().onmessage?.({ data: raw });
}

function receive(message: { type: string; data?: unknown }): void {
    receiveRaw(JSON.stringify(message));
}

function reportedMessages(): string[] {
    return reportClientError.mock.calls.map(([error]) => (error instanceof Error ? error.message : String(error)));
}

beforeEach(async () => {
    FakeWebSocket.instances = [];
    vi.stubGlobal("WebSocket", FakeWebSocket);
    reportClientError.mockClear();
    vi.resetModules();
    bus = await import("./bus");
    pipeline = await import("./pipeline");
    socket = await import("./socket");
});

afterEach(() => {
    vi.restoreAllMocks();
});

describe("realtime pipeline delivery", () => {
    it("hands a packet off the socket to a bus subscriber under its declared name", () => {
        // given
        const handler = vi.fn();
        bus.subscribe("pong", handler);
        connect();

        // when
        receive({ type: "pong", data: { nonce: 3, server_now: SERVER_NOW } });

        // then
        expect(handler).toHaveBeenCalledTimes(1);
        expect(handler).toHaveBeenCalledWith({ type: "pong", data: { nonce: 3, server_now: SERVER_NOW } });
    });

    it("dispatches nothing when the packet is not json the parser accepts", () => {
        // given
        const handler = vi.fn();
        bus.subscribe("pong", handler);
        connect();

        // when
        receiveRaw('{"type":"pong","data":');

        // then
        expect(handler).not.toHaveBeenCalled();
    });

    it("dispatches nothing when the type is not a declared event", () => {
        // given
        const handler = vi.fn();
        bus.subscribe("pong", handler);
        connect();

        // when
        receive({ type: "totally_made_up", data: { nonce: 3 } });

        // then
        expect(handler).not.toHaveBeenCalled();
    });

    it("leaves a subscriber that has let go alone", () => {
        // given
        const handler = vi.fn();
        const unsubscribe = bus.subscribe("pong", handler);
        connect();
        unsubscribe();

        // when
        receive({ type: "pong", data: { nonce: 3 } });

        // then
        expect(handler).not.toHaveBeenCalled();
    });

    it("installs the frame handler once, however many holders ask for the pipeline", () => {
        // given
        const handler = vi.fn();
        bus.subscribe("pong", handler);
        connect();

        // when
        pipeline.ensureRealtimePipeline();
        pipeline.ensureRealtimePipeline();
        receive({ type: "pong", data: { nonce: 3 } });

        // then
        expect(handler).toHaveBeenCalledTimes(1);
    });
});

describe("realtime pipeline guards", () => {
    it("drops a pong whose server clock is not a string, because the timer offset is derived from it", () => {
        // given
        const handler = vi.fn();
        bus.subscribe("pong", handler);
        connect();

        // when
        receive({ type: "pong", data: { nonce: 1, server_now: 1727000000000 } });

        // then
        expect(handler).not.toHaveBeenCalled();
    });

    it("drops a pong whose nonce is not a finite number", () => {
        // given
        const handler = vi.fn();
        bus.subscribe("pong", handler);
        connect();

        // when
        receive({ type: "pong", data: { nonce: "1", server_now: SERVER_NOW } });

        // then
        expect(handler).not.toHaveBeenCalled();
    });

    it("delivers a guarded event whose payload is well formed", () => {
        // given
        const handler = vi.fn();
        bus.subscribe("pong", handler);
        connect();
        const data = { nonce: 1, server_now: SERVER_NOW };

        // when
        receive({ type: "pong", data });

        // then
        expect(handler).toHaveBeenCalledWith({ type: "pong", data });
    });

    it("delivers a well formed board frame to the board's subscriber", () => {
        // given
        const handler = vi.fn();
        bus.subscribe("board_events", handler);
        connect();
        const data = {
            board_id: "8f0c",
            from: 1042,
            to: 1042,
            events: [{ type: "card_deleted", version: 1042, card_id: "c1e2" }],
        };

        // when
        receive({ type: "board_events", data });

        // then
        expect(handler).toHaveBeenCalledWith({ type: "board_events", data });
    });

    it("drops a board frame with an event it cannot apply, and reports it, so the next frame's gap triggers a refetch", () => {
        // given
        const handler = vi.fn();
        bus.subscribe("board_events", handler);
        connect();

        // when
        receive({
            type: "board_events",
            data: { board_id: "8f0c", from: 1042, to: 1042, events: [{ type: "votes_changed", version: 1042 }] },
        });

        // then
        expect(handler).not.toHaveBeenCalled();
        expect(reportedMessages()).toEqual(['realtime event "board_events" failed its payload guard']);
    });
});

describe("a frame the contract rejects is never dropped without a trace", () => {
    it("reports each parse failure reason once per page load", () => {
        // given
        connect();

        // when
        receiveRaw('{"type":"pong",');
        receiveRaw("not json");
        receiveRaw("42");
        receiveRaw('[{"type":"pong"}]');
        receiveRaw('{"data":{}}');
        receiveRaw('{"type":7}');

        // then
        expect(reportedMessages()).toEqual([
            "realtime frame rejected: invalid-json",
            "realtime frame rejected: not-an-object",
            "realtime frame rejected: missing-type",
        ]);
    });

    it("reports an unknown event type once per type", () => {
        // given
        connect();

        // when
        receive({ type: "an_event_nobody_declared", data: {} });
        receive({ type: "an_event_nobody_declared", data: {} });
        receive({ type: "another_event_nobody_declared", data: {} });
        receive({ type: "another_event_nobody_declared", data: {} });

        // then
        expect(reportedMessages()).toEqual([
            'unknown realtime event "an_event_nobody_declared"',
            'unknown realtime event "another_event_nobody_declared"',
        ]);
    });

    it("reports an event type failing its guard once per type", () => {
        // given
        connect();

        // when
        receive({ type: "pong", data: { server_now: 1 } });
        receive({ type: "pong", data: { nonce: "x" } });
        receive({ type: "pong", data: null });

        // then
        expect(reportedMessages()).toEqual(['realtime event "pong" failed its payload guard']);
    });

    it("reports every kind of rejection under the realtime contract source", () => {
        // given
        connect();

        // when
        receiveRaw("not json");
        receive({ type: "an_event_nobody_declared", data: {} });
        receive({ type: "pong", data: { nonce: "x" } });

        // then
        expect(reportClientError).toHaveBeenCalledTimes(3);
        for (const [error, context] of reportClientError.mock.calls) {
            expect(error).toBeInstanceOf(Error);
            expect(context).toEqual({ source: "realtime-contract" });
        }
    });

    it("names the failure, never the frame's contents", () => {
        // given
        connect();

        // when
        receiveRaw('{"type":"pong","data":{"body":"SECRET-TEXT-9f2c"}');
        receive({ type: "pong", data: { nonce: "SECRET-TEXT-9f2c" } });

        // then
        expect(reportedMessages()).toHaveLength(2);
        for (const message of reportedMessages()) {
            expect(message).not.toContain("SECRET-TEXT-9f2c");
        }
    });

    it("reports nothing for a frame that parses and passes its guard", () => {
        // given
        connect();

        // when
        receive({ type: "pong", data: { nonce: 1, server_now: SERVER_NOW } });
        receive({ type: "pong", data: {} });

        // then
        expect(reportClientError).not.toHaveBeenCalled();
    });

    it("still dispatches nothing for an unknown event, so it cannot reach a handler", () => {
        // given
        const handler = vi.fn();
        bus.subscribe(["pong"], handler);
        connect();

        // when
        receive({ type: "an_event_nobody_declared", data: {} });

        // then
        expect(handler).not.toHaveBeenCalled();
    });
});

describe("realtime reconnect epoch", () => {
    it("starts at zero before the socket has ever opened", () => {
        // given
        pipeline.ensureRealtimePipeline();

        // then
        expect(pipeline.getRealtimeEpoch()).toBe(0);
    });

    it("counts one for the first connection", () => {
        // when
        connect();

        // then
        expect(pipeline.getRealtimeEpoch()).toBe(1);
    });

    it("does not move while messages are arriving", () => {
        // given
        connect();

        // when
        receive({ type: "pong", data: { nonce: 1 } });
        receive({ type: "pong", data: { nonce: 2 } });
        receive({ type: "pong", data: { nonce: 3 } });

        // then
        expect(pipeline.getRealtimeEpoch()).toBe(1);
    });

    it("counts one more for a reconnect", () => {
        // given
        vi.useFakeTimers();
        vi.spyOn(Math, "random").mockReturnValue(1);
        connect();

        // when
        lastSocket().onclose?.();
        vi.advanceTimersByTime(1000);
        lastSocket().onopen?.();

        // then
        expect(pipeline.getRealtimeEpoch()).toBe(2);
    });

    it("notifies an epoch subscriber once per connection", () => {
        // given
        const listener = vi.fn();
        pipeline.subscribeRealtimeEpoch(listener);

        // when
        connect();
        receive({ type: "pong", data: { nonce: 1 } });

        // then
        expect(listener).toHaveBeenCalledTimes(1);
    });

    it("reports the socket as connected while open and not once it closes", () => {
        // given
        vi.useFakeTimers();
        const listener = vi.fn();
        pipeline.subscribeRealtimeEpoch(listener);
        connect();
        expect(pipeline.getRealtimeConnected()).toBe(true);

        // when
        lastSocket().onclose?.();

        // then
        expect(pipeline.getRealtimeConnected()).toBe(false);
        expect(pipeline.getRealtimeEpoch()).toBe(1);
        expect(listener).toHaveBeenCalledTimes(2);
    });

    it("stops notifying an epoch subscriber that has unsubscribed", () => {
        // given
        const listener = vi.fn();
        const unsubscribe = pipeline.subscribeRealtimeEpoch(listener);

        // when
        unsubscribe();
        connect();

        // then
        expect(listener).not.toHaveBeenCalled();
        expect(pipeline.getRealtimeEpoch()).toBe(1);
    });
});
