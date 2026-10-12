import { describe, expect, it } from "vitest";
import { POPUP_GAP, popupPosition } from "./popupPosition";

const VIEWPORT = { width: 360, height: 640 };

describe("popupPosition", () => {
    it("opens below the anchor, lined up with its end edge", () => {
        // given
        const anchor = { top: 100, bottom: 132, left: 280, right: 320 };

        // when
        const position = popupPosition(anchor, 120, VIEWPORT, false);

        // then
        expect(position).toEqual({ top: 132 + POPUP_GAP, insetInlineEnd: 40 });
    });

    it("flips above the anchor when it would run off the bottom of the viewport", () => {
        // given
        const anchor = { top: 580, bottom: 612, left: 280, right: 320 };

        // when
        const position = popupPosition(anchor, 120, VIEWPORT, false);

        // then
        expect(position).toEqual({ top: 580 - POPUP_GAP - 120, insetInlineEnd: 40 });
    });

    it("stays below when there is no space above either", () => {
        // given
        const anchor = { top: 60, bottom: 92, left: 280, right: 320 };

        // when
        const position = popupPosition(anchor, 600, VIEWPORT, false);

        // then
        expect(position.top).toBe(92 + POPUP_GAP);
    });

    it("measures the end edge from the left in a right-to-left page", () => {
        // given
        const anchor = { top: 100, bottom: 132, left: 24, right: 64 };

        // when
        const position = popupPosition(anchor, 120, VIEWPORT, true);

        // then
        expect(position.insetInlineEnd).toBe(24);
    });
});
