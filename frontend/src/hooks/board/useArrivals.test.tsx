import { renderHook } from "@testing-library/react";
import type { ReactNode } from "react";
import { describe, expect, it } from "vitest";
import { ArrivalsContext, type Arrivals, useArrival, useArrivalTracker } from "./useArrivals";

function withArrivals(arrivals: Arrivals) {
    return function Wrapper({ children }: { children: ReactNode }) {
        return <ArrivalsContext.Provider value={arrivals}>{children}</ArrivalsContext.Provider>;
    };
}

describe("useArrivalTracker", () => {
    it("counts nothing already on the board when it opened as new", () => {
        // when
        const { result } = renderHook(() => useArrivalTracker(["k1", "k2"]));

        // then
        expect(result.current.isNew("k1")).toBe(false);
        expect(result.current.isNew("k2")).toBe(false);
    });

    it("counts a card that turns up later as new, once", () => {
        // given
        const { result, rerender } = renderHook(({ keys }) => useArrivalTracker(keys), {
            initialProps: { keys: ["k1"] },
        });

        // when
        const before = result.current.isNew("k2");
        rerender({ keys: ["k1", "k2"] });
        const after = result.current.isNew("k2");

        // then the new card is new until it has been shown, then never again
        expect(before).toBe(true);
        expect(after).toBe(false);
    });
});

describe("useArrival", () => {
    it("decides once, when the card first appears, and keeps that answer", () => {
        // given
        let fresh = true;
        const arrivals: Arrivals = { isNew: () => fresh };
        const { result, rerender } = renderHook(() => useArrival("k9"), { wrapper: withArrivals(arrivals) });

        // when
        fresh = false;
        rerender();

        // then
        expect(result.current).toBe(true);
    });

    it("never animates outside a board", () => {
        // when
        const { result } = renderHook(() => useArrival("k9"));

        // then
        expect(result.current).toBe(false);
    });
});
