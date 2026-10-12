import { act, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type * as OutboundModule from "../api/realtime/outbound";
import { emitRealtimeEvent, makeWSHarness, type WSHarness } from "../test-utils/ws";
import { useServerPing } from "./useServerPing";

const holder = vi.hoisted(() => ({ ws: null as unknown as WSHarness }));

vi.mock("../api/realtime/pipeline", () => ({
    ensureRealtimePipeline: () => {},
    getRealtimeEpoch: () => holder.ws.getEpoch(),
    subscribeRealtimeEpoch: (listener: () => void) => holder.ws.subscribeEpoch(listener),
}));

vi.mock("../api/realtime/outbound", async importOriginal => {
    const actual = await importOriginal<typeof OutboundModule>();

    return { ...actual, sendRealtime: (command: OutboundModule.RealtimeCommand) => holder.ws.sendRealtime(command) };
});

const WALL_CLOCK = Date.parse("2026-09-22T10:06:00.000Z");

let clock = 0;

function sentPings(): OutboundModule.PingPayload[] {
    return holder.ws.sendRealtime.mock.calls
        .map(([command]) => command)
        .filter(command => command.type === "ping")
        .map(command => command.data as OutboundModule.PingPayload);
}

function sentNonces(): (number | undefined)[] {
    return sentPings().map(ping => ping.nonce);
}

function answer(nonce: number, afterMs: number, serverNow?: string): void {
    clock += afterMs;
    emitRealtimeEvent({ type: "pong", data: serverNow === undefined ? { nonce } : { nonce, server_now: serverNow } });
}

beforeEach(() => {
    holder.ws = makeWSHarness();
    clock = 0;
    vi.useFakeTimers();
    vi.setSystemTime(WALL_CLOCK);
    vi.stubGlobal("performance", { now: () => clock });
});

afterEach(() => {
    vi.useRealTimers();
    vi.unstubAllGlobals();
});

describe("useServerPing", () => {
    it("probes the server as soon as it is enabled", () => {
        // when
        renderHook(() => useServerPing(true));

        // then
        expect(sentNonces()).toEqual([1]);
    });

    it("stays silent while it is disabled", () => {
        // given
        renderHook(() => useServerPing(false));

        // when
        act(() => {
            vi.advanceTimersByTime(5000);
        });

        // then
        expect(sentNonces()).toEqual([]);
    });

    it("keeps probing on a timer", () => {
        // given
        renderHook(() => useServerPing(true));

        // when
        act(() => {
            vi.advanceTimersByTime(2000);
        });

        // then
        expect(sentNonces()).toEqual([1, 2, 3]);
    });

    it("reports the round trip of the reply to its own probe", () => {
        // given
        const { result } = renderHook(() => useServerPing(true));

        // when
        answer(1, 40);

        // then
        expect(result.current.roundTripMs).toBe(40);
        expect(result.current.grade).toBe("good");
    });

    it("tells the server the round trip it measured on the next probe", () => {
        // given
        renderHook(() => useServerPing(true));
        answer(1, 40);

        // when
        act(() => {
            vi.advanceTimersByTime(1000);
        });

        // then
        expect(sentPings().at(-1)).toEqual({ nonce: 2, rtt: 40 });
    });

    it("ignores a pong it never asked for, so the keepalive cannot fake a fast link", () => {
        // given
        const { result } = renderHook(() => useServerPing(true));

        // when
        answer(999, 1);

        // then
        expect(result.current.roundTripMs).toBeNull();
    });

    it("ignores a pong that carries no nonce", () => {
        // given
        const { result } = renderHook(() => useServerPing(true));

        // when
        clock += 5;
        emitRealtimeEvent({ type: "pong", data: {} });

        // then
        expect(result.current.roundTripMs).toBeNull();
    });

    it("reports the median rather than the latest sample", () => {
        // given
        const { result } = renderHook(() => useServerPing(true));

        // when
        answer(1, 200);
        act(() => {
            vi.advanceTimersByTime(1000);
        });
        answer(2, 200);
        act(() => {
            vi.advanceTimersByTime(1000);
        });
        answer(3, 900);

        // then
        expect(result.current.roundTripMs).toBe(200);
    });

    it("grades a distant link as poor", () => {
        // given
        const { result } = renderHook(() => useServerPing(true));

        // when
        answer(1, 240);

        // then
        expect(result.current.grade).toBe("poor");
    });

    it("works out the server's clock offset from server_now plus half the round trip", () => {
        // given
        const { result } = renderHook(() => useServerPing(true));
        const serverAhead = new Date(WALL_CLOCK + 2_000).toISOString();

        // when
        answer(1, 100, serverAhead);

        // then
        expect(result.current.clockOffsetMs).toBe(2_050);
    });

    it("has no clock offset until a pong carries the server's clock", () => {
        // given
        const { result } = renderHook(() => useServerPing(true));

        // when
        answer(1, 40);

        // then
        expect(result.current.roundTripMs).toBe(40);
        expect(result.current.clockOffsetMs).toBeNull();
    });

    it("probes afresh after the socket reconnects", () => {
        // given
        renderHook(() => useServerPing(true));

        // when
        holder.ws.reconnect();

        // then
        expect(sentNonces()).toEqual([1, 2]);
    });

    it("stops probing once it is torn down", () => {
        // given
        const { unmount } = renderHook(() => useServerPing(true));

        // when
        unmount();
        act(() => {
            vi.advanceTimersByTime(5000);
        });

        // then
        expect(sentNonces()).toEqual([1]);
    });
});
