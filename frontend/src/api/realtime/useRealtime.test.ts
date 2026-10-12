import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { FakeWebSocket } from "../../test-utils/ws";
import type * as BusModule from "./bus";
import type { RealtimeEventName } from "./events";
import type * as SocketModule from "./socket";
import type * as UseRealtimeModule from "./useRealtime";

const { subscribeCalls } = vi.hoisted(() => ({
    subscribeCalls: { count: 0 },
}));

vi.mock("./bus", async importOriginal => {
    const actual = await importOriginal<typeof BusModule>();

    function countingSubscribe<K extends RealtimeEventName>(
        names: K | readonly K[],
        handler: BusModule.RealtimeEventHandler<K>,
    ): BusModule.RealtimeUnsubscribe {
        subscribeCalls.count += 1;

        return actual.subscribe(names, handler);
    }

    return { ...actual, subscribe: countingSubscribe };
});

let hooks: typeof UseRealtimeModule;
let socket: typeof SocketModule;

function lastSocket(): FakeWebSocket {
    const instance = FakeWebSocket.instances[FakeWebSocket.instances.length - 1];
    if (!instance) {
        throw new Error("no websocket was opened");
    }

    return instance;
}

function connect(): void {
    act(() => {
        socket.openRealtimeSocket("user:u1");
        lastSocket().onopen?.();
    });
}

function reconnect(): void {
    act(() => {
        socket.closeRealtimeSocket("user:u1");
        socket.openRealtimeSocket("user:u1");
        lastSocket().onopen?.();
    });
}

function receive(message: { type: string; data?: unknown }): void {
    act(() => {
        lastSocket().onmessage?.({ data: JSON.stringify(message) });
    });
}

beforeEach(async () => {
    FakeWebSocket.instances = [];
    subscribeCalls.count = 0;
    vi.stubGlobal("WebSocket", FakeWebSocket);
    vi.resetModules();
    socket = await import("./socket");
    hooks = await import("./useRealtime");
});

describe("useRealtimeEvent", () => {
    it("delivers an event straight off the socket, having opened the pipeline itself", () => {
        // given
        const handler = vi.fn();
        renderHook(() => {
            hooks.useRealtimeEvent("pong", handler);
        });
        connect();

        // when
        receive({ type: "pong", data: { nonce: 3 } });

        // then
        expect(handler).toHaveBeenCalledTimes(1);
        expect(handler).toHaveBeenCalledWith({ type: "pong", data: { nonce: 3 } });
    });

    it("hands the event to the handler from the latest render, not the one that subscribed", () => {
        // given
        const seen: number[] = [];
        const { rerender } = renderHook(
            ({ value }: { value: number }) => {
                hooks.useRealtimeEvent("pong", () => {
                    seen.push(value);
                });
            },
            { initialProps: { value: 1 } },
        );
        connect();

        // when
        rerender({ value: 2 });
        rerender({ value: 3 });
        receive({ type: "pong", data: { nonce: 7 } });

        // then
        expect(seen).toEqual([3]);
    });

    it("does not resubscribe when a re-render brings a new array literal and a new handler closure", () => {
        // given
        const handler = vi.fn();
        const { rerender } = renderHook(
            ({ value }: { value: number }) => {
                hooks.useRealtimeEvent(["pong"], () => {
                    handler(value);
                });
            },
            { initialProps: { value: 1 } },
        );
        connect();

        // when
        rerender({ value: 2 });
        rerender({ value: 3 });
        rerender({ value: 4 });
        receive({ type: "pong", data: { nonce: 1 } });

        // then
        expect(subscribeCalls.count).toBe(1);
        expect(handler).toHaveBeenCalledWith(4);
    });

    it("resubscribes when the wanted names really change, and lets go of the old subscription", () => {
        // given
        const handler = vi.fn();
        const { rerender } = renderHook(
            ({ names }: { names: RealtimeEventName | readonly RealtimeEventName[] }) => {
                hooks.useRealtimeEvent(names, handler);
            },
            { initialProps: { names: "pong" as RealtimeEventName | readonly RealtimeEventName[] } },
        );
        connect();

        // when
        rerender({ names: ["pong", "pong"] });
        receive({ type: "pong", data: { nonce: 1 } });

        // then
        expect(subscribeCalls.count).toBe(2);
        expect(handler).toHaveBeenCalledTimes(1);
        expect(handler).toHaveBeenCalledWith({ type: "pong", data: { nonce: 1 } });
    });

    it("stops delivering once the consumer unmounts", () => {
        // given
        const handler = vi.fn();
        const view = renderHook(() => {
            hooks.useRealtimeEvent("pong", handler);
        });
        connect();

        // when
        view.unmount();
        receive({ type: "pong", data: { nonce: 3 } });

        // then
        expect(handler).not.toHaveBeenCalled();
    });
});

describe("useRealtimeStatus", () => {
    it("reports zero until the socket has connected", () => {
        // given
        const { result } = renderHook(() => hooks.useRealtimeStatus());

        // then
        expect(result.current).toBe(0);
    });

    it("moves once when the socket connects", () => {
        // given
        const { result } = renderHook(() => hooks.useRealtimeStatus());

        // when
        connect();

        // then
        expect(result.current).toBe(1);
    });

    it("keeps the same value across messages, so a consumer effect keyed on it does not refire", () => {
        // given
        const seen: number[] = [];
        renderHook(() => {
            seen.push(hooks.useRealtimeStatus());
        });
        connect();

        // when
        receive({ type: "pong", data: { nonce: 1 } });
        receive({ type: "pong", data: { nonce: 2 } });
        receive({ type: "pong", data: { nonce: 3 } });

        // then
        expect(seen).toEqual([0, 1]);
    });

    it("moves exactly once more when the socket connects again", () => {
        // given
        const { result } = renderHook(() => hooks.useRealtimeStatus());
        connect();

        // when
        reconnect();

        // then
        expect(result.current).toBe(2);
    });
});

describe("the socket the hooks listen on", () => {
    it("connects to the socket route on the page's own origin, with no query string", () => {
        // given
        renderHook(() => hooks.useRealtimeStatus());

        // when
        connect();

        // then
        expect(lastSocket().url).toBe("ws://localhost:3000/api/v1/ws");
    });
});
