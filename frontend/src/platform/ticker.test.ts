import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { getTickerNow, subscribeTicker, TICK_MS } from "./ticker";

const START = Date.parse("2026-10-01T09:30:00.000Z");

beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(START);
});

afterEach(() => {
    vi.useRealTimers();
});

describe("the ticker", () => {
    it("reads the clock as soon as someone subscribes", () => {
        // when
        const unsubscribe = subscribeTicker(vi.fn());

        // then
        expect(getTickerNow()).toBe(START);
        unsubscribe();
    });

    it("tells every subscriber on each tick and moves the clock on", () => {
        // given
        const first = vi.fn();
        const second = vi.fn();
        const stopFirst = subscribeTicker(first);
        const stopSecond = subscribeTicker(second);

        // when
        vi.advanceTimersByTime(TICK_MS * 2);

        // then
        expect(first).toHaveBeenCalledTimes(2);
        expect(second).toHaveBeenCalledTimes(2);
        expect(getTickerNow()).toBe(START + TICK_MS * 2);
        stopFirst();
        stopSecond();
    });

    it("stops ticking once the last subscriber leaves", () => {
        // given
        const listener = vi.fn();
        const unsubscribe = subscribeTicker(listener);

        // when
        unsubscribe();
        vi.advanceTimersByTime(TICK_MS * 4);

        // then
        expect(listener).not.toHaveBeenCalled();
        expect(vi.getTimerCount()).toBe(0);
    });
});
