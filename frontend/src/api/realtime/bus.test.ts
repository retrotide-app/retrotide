import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type * as BusModule from "./bus";
import { REALTIME_EVENTS, type RealtimeEvent, type RealtimeEventName } from "./events";

let bus: typeof BusModule;
let windowErrors: unknown[] = [];

const nativeQueueMicrotask = queueMicrotask;

function captureWindowError(event: ErrorEvent): void {
    event.preventDefault();
    windowErrors.push(event.error);
}

function emulateBrowserErrorReporting(): void {
    vi.stubGlobal("queueMicrotask", (callback: VoidFunction) => {
        nativeQueueMicrotask(() => {
            try {
                callback();
            } catch (error) {
                window.dispatchEvent(new ErrorEvent("error", { error, message: String(error), cancelable: true }));
            }
        });
    });
}

function pongEvent(nonce: number): RealtimeEvent {
    return { type: "pong", data: { nonce, server_now: "2026-09-22T10:06:02.512Z" } };
}

beforeEach(async () => {
    windowErrors = [];
    window.addEventListener("error", captureWindowError);
    emulateBrowserErrorReporting();
    vi.resetModules();
    bus = await import("./bus");
});

afterEach(() => {
    window.removeEventListener("error", captureWindowError);
});

describe("realtime bus delivery", () => {
    it("hands an event to a handler subscribed to its name", () => {
        // given
        const handler = vi.fn();
        bus.subscribe("pong", handler);
        const event = pongEvent(1);

        // when
        bus.dispatch(event);

        // then
        expect(handler).toHaveBeenCalledTimes(1);
        expect(handler).toHaveBeenCalledWith(event);
    });

    it("hands the event to every handler subscribed to its name", () => {
        // given
        const first = vi.fn();
        const second = vi.fn();
        bus.subscribe("pong", first);
        bus.subscribe("pong", second);
        const event = pongEvent(1);

        // when
        bus.dispatch(event);

        // then
        expect(first).toHaveBeenCalledWith(event);
        expect(second).toHaveBeenCalledWith(event);
    });

    it("accepts a list of names as well as a single name", () => {
        // given
        const handler = vi.fn();
        bus.subscribe(["pong"], handler);
        const first = pongEvent(1);
        const second = pongEvent(2);

        // when
        bus.dispatch(first);
        bus.dispatch(second);

        // then
        expect(handler.mock.calls).toEqual([[first], [second]]);
    });

    it("delivers once to a handler that named the same event twice", () => {
        // given
        const handler = vi.fn();
        bus.subscribe(["pong", "pong"], handler);

        // when
        bus.dispatch(pongEvent(1));

        // then
        expect(handler).toHaveBeenCalledTimes(1);
    });

    it("subscribes to a name that is only known at runtime", () => {
        // given
        const handler = vi.fn();
        const wanted: RealtimeEventName = REALTIME_EVENTS.PONG;
        bus.subscribe(wanted, handler);
        const event = pongEvent(1);

        // when
        bus.dispatch(event);

        // then
        expect(handler).toHaveBeenCalledWith(event);
    });

    it("ignores an event nobody is subscribed to", () => {
        // given
        const handler = vi.fn();
        const unsubscribe = bus.subscribe("pong", handler);
        unsubscribe();

        // when
        bus.dispatch(pongEvent(1));

        // then
        expect(handler).not.toHaveBeenCalled();
        expect(windowErrors).toEqual([]);
    });
});

describe("realtime bus handler isolation", () => {
    it("does not stop the handlers registered after a throwing one", async () => {
        // given
        const failure = new Error("handler is broken");
        const throwing = vi.fn(() => {
            throw failure;
        });
        const second = vi.fn();
        const third = vi.fn();
        bus.subscribe("pong", throwing);
        bus.subscribe("pong", second);
        bus.subscribe("pong", third);
        const event = pongEvent(1);

        // when
        bus.dispatch(event);
        await Promise.resolve();

        // then
        expect(throwing).toHaveBeenCalledTimes(1);
        expect(second).toHaveBeenCalledWith(event);
        expect(third).toHaveBeenCalledWith(event);
        expect(windowErrors).toEqual([failure]);
    });

    it("rethrows a handler failure where a window error listener sees it", async () => {
        // given
        const failure = new Error("handler is broken");
        bus.subscribe("pong", () => {
            throw failure;
        });

        // when
        bus.dispatch(pongEvent(1));

        // then
        expect(windowErrors).toEqual([]);
        await Promise.resolve();
        expect(windowErrors).toEqual([failure]);
    });

    it("keeps a handler that threw subscribed for the next event", async () => {
        // given
        const failure = new Error("handler is broken");
        const throwing = vi.fn(() => {
            throw failure;
        });
        bus.subscribe("pong", throwing);

        // when
        bus.dispatch(pongEvent(1));
        bus.dispatch(pongEvent(2));
        await Promise.resolve();

        // then
        expect(throwing).toHaveBeenCalledTimes(2);
        expect(windowErrors).toEqual([failure, failure]);
    });
});

describe("realtime bus subscription lifetime", () => {
    it("stops delivering once the subscriber has unsubscribed", () => {
        // given
        const handler = vi.fn();
        const unsubscribe = bus.subscribe(["pong"], handler);

        // when
        unsubscribe();
        bus.dispatch(pongEvent(1));

        // then
        expect(handler).not.toHaveBeenCalled();
    });

    it("keeps delivering to the other subscribers when one lets go", () => {
        // given
        const leaving = vi.fn();
        const staying = vi.fn();
        const unsubscribe = bus.subscribe("pong", leaving);
        bus.subscribe("pong", staying);

        // when
        unsubscribe();
        bus.dispatch(pongEvent(1));

        // then
        expect(leaving).not.toHaveBeenCalled();
        expect(staying).toHaveBeenCalledTimes(1);
    });

    it("ignores a second call to the same unsubscribe", () => {
        // given
        const handler = vi.fn();
        const stale = bus.subscribe("pong", handler);
        stale();
        bus.subscribe("pong", handler);

        // when
        stale();
        bus.dispatch(pongEvent(1));

        // then
        expect(handler).toHaveBeenCalledTimes(1);
    });

    it("does not change who receives the current event when a handler unsubscribes during dispatch", () => {
        // given
        const second = vi.fn();
        const third = vi.fn();
        const unsubscribers: BusModule.RealtimeUnsubscribe[] = [];
        bus.subscribe("pong", () => {
            for (const unsubscribe of unsubscribers) {
                unsubscribe();
            }
        });
        unsubscribers.push(bus.subscribe("pong", second));
        unsubscribers.push(bus.subscribe("pong", third));
        const event = pongEvent(1);

        // when
        bus.dispatch(event);

        // then
        expect(second).toHaveBeenCalledWith(event);
        expect(third).toHaveBeenCalledWith(event);

        bus.dispatch(pongEvent(2));
        expect(second).toHaveBeenCalledTimes(1);
        expect(third).toHaveBeenCalledTimes(1);
    });

    it("holds the current event back from a handler that subscribes during dispatch", () => {
        // given
        const latecomer = vi.fn();
        bus.subscribe("pong", () => {
            bus.subscribe("pong", latecomer);
        });

        // when
        bus.dispatch(pongEvent(1));

        // then
        expect(latecomer).not.toHaveBeenCalled();

        bus.dispatch(pongEvent(2));
        expect(latecomer).toHaveBeenCalledTimes(1);
    });
});
