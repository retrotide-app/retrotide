import { renderHook } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { createCardFocus, useCardFocus } from "./useCardFocus";

describe("createCardFocus", () => {
    it("hands focus to the requested card once", () => {
        // given
        const cardFocus = createCardFocus();
        cardFocus.request("k1");

        // when
        const claims = [cardFocus.claim("k2"), cardFocus.claim("k1"), cardFocus.claim("k1")];

        // then
        expect(claims).toEqual([false, true, false]);
    });

    it("hands focus to no card once the request is withdrawn", () => {
        // given
        const cardFocus = createCardFocus();
        cardFocus.request("k1");

        // when
        cardFocus.request(null);

        // then
        expect(cardFocus.claim("k1")).toBe(false);
    });

    it("reveals a card on the board through the card's watcher", () => {
        // given
        const cardFocus = createCardFocus();
        const onReveal = vi.fn();
        cardFocus.watch("k1", onReveal);

        // when
        const revealed = [cardFocus.reveal("k1"), cardFocus.reveal("k2")];

        // then
        expect(revealed).toEqual([true, false]);
        expect(onReveal).toHaveBeenCalledTimes(1);
    });

    it("reveals nothing once the card stops watching", () => {
        // given
        const cardFocus = createCardFocus();
        const onReveal = vi.fn();
        const stop = cardFocus.watch("k1", onReveal);

        // when
        stop();

        // then
        expect(cardFocus.reveal("k1")).toBe(false);
        expect(onReveal).not.toHaveBeenCalled();
    });

    it("keeps the newer watcher when an older one stops", () => {
        // given
        const cardFocus = createCardFocus();
        const older = vi.fn();
        const newer = vi.fn();
        const stopOlder = cardFocus.watch("k1", older);
        cardFocus.watch("k1", newer);

        // when
        stopOlder();
        cardFocus.reveal("k1");

        // then
        expect(older).not.toHaveBeenCalled();
        expect(newer).toHaveBeenCalledTimes(1);
    });
});

describe("useCardFocus", () => {
    it("keeps the same request across renders", () => {
        // given
        const { result, rerender } = renderHook(() => useCardFocus());
        result.current.request("k1");

        // when
        rerender();

        // then
        expect(result.current.claim("k1")).toBe(true);
    });
});
