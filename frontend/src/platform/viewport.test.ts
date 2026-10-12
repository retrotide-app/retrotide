import { afterEach, describe, expect, it, vi } from "vitest";
import { isNarrowViewport, NARROW_VIEWPORT_QUERY, subscribeNarrowViewport } from "./viewport";

function fakeMedia(matches: boolean) {
    return {
        matches,
        media: NARROW_VIEWPORT_QUERY,
        onchange: null,
        addListener: vi.fn(),
        removeListener: vi.fn(),
        addEventListener: vi.fn(),
        removeEventListener: vi.fn(),
        dispatchEvent: vi.fn(() => false),
    };
}

afterEach(() => {
    vi.restoreAllMocks();
});

describe("isNarrowViewport", () => {
    it.each([true, false])("answers whether the phone-width query matches: %s", matches => {
        // given
        const matchMedia = vi.spyOn(window, "matchMedia").mockReturnValue(fakeMedia(matches));

        // when
        const narrow = isNarrowViewport();

        // then
        expect(narrow).toBe(matches);
        expect(matchMedia).toHaveBeenCalledWith(NARROW_VIEWPORT_QUERY);
    });
});

describe("subscribeNarrowViewport", () => {
    it("listens for the query to change and stops when unsubscribed", () => {
        // given
        const media = fakeMedia(false);
        vi.spyOn(window, "matchMedia").mockReturnValue(media);
        const onChange = vi.fn();

        // when
        const unsubscribe = subscribeNarrowViewport(onChange);
        unsubscribe();

        // then
        expect(media.addEventListener).toHaveBeenCalledWith("change", onChange);
        expect(media.removeEventListener).toHaveBeenCalledWith("change", onChange);
    });
});
