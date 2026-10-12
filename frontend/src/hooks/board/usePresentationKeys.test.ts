import { fireEvent, renderHook } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { type PresentationKeyHandlers, usePresentationKeys } from "./usePresentationKeys";

function handlers(overrides: Partial<PresentationKeyHandlers> = {}): PresentationKeyHandlers {
    return {
        presenting: true,
        steers: true,
        next: "k3",
        previous: "k1",
        onPresent: vi.fn(),
        onLeave: vi.fn(),
        ...overrides,
    };
}

afterEach(() => {
    for (const planted of document.querySelectorAll("[data-planted]")) {
        planted.remove();
    }
});

describe("usePresentationKeys", () => {
    it("steps forward, back and ends the presentation for the facilitator", () => {
        // given
        const keys = handlers();
        renderHook(() => usePresentationKeys(keys));

        // when
        fireEvent.keyDown(window, { key: "ArrowRight" });
        fireEvent.keyDown(window, { key: "ArrowLeft" });
        fireEvent.keyDown(window, { key: "Escape" });

        // then
        expect(keys.onPresent).toHaveBeenNthCalledWith(1, { cardId: "k3" });
        expect(keys.onPresent).toHaveBeenNthCalledWith(2, { cardId: "k1" });
        expect(keys.onPresent).toHaveBeenNthCalledWith(3, { cardId: null });
        expect(keys.onLeave).not.toHaveBeenCalled();
    });

    it("takes no step past either end of the order", () => {
        // given
        const keys = handlers({ next: null, previous: null });
        renderHook(() => usePresentationKeys(keys));

        // when
        fireEvent.keyDown(window, { key: "ArrowRight" });
        fireEvent.keyDown(window, { key: "ArrowLeft" });

        // then
        expect(keys.onPresent).not.toHaveBeenCalled();
    });

    it("lets a follower's Escape leave only their own view, and gives them no steps", () => {
        // given
        const keys = handlers({ steers: false });
        renderHook(() => usePresentationKeys(keys));

        // when
        fireEvent.keyDown(window, { key: "ArrowRight" });
        fireEvent.keyDown(window, { key: "ArrowLeft" });
        fireEvent.keyDown(window, { key: "Escape" });

        // then
        expect(keys.onLeave).toHaveBeenCalledOnce();
        expect(keys.onPresent).not.toHaveBeenCalled();
    });

    it("ignores the keys while nothing is presented", () => {
        // given
        const keys = handlers({ presenting: false });
        renderHook(() => usePresentationKeys(keys));

        // when
        fireEvent.keyDown(window, { key: "ArrowRight" });
        fireEvent.keyDown(window, { key: "Escape" });

        // then
        expect(keys.onPresent).not.toHaveBeenCalled();
        expect(keys.onLeave).not.toHaveBeenCalled();
    });

    it.each([
        ["an input", "input"],
        ["a text area", "textarea"],
        ["a dialog", "dialog"],
    ])("ignores keystrokes typed in %s", (_, kind) => {
        // given
        const keys = handlers();
        renderHook(() => usePresentationKeys(keys));
        const host = document.createElement(kind === "dialog" ? "div" : kind);
        host.setAttribute("data-planted", "");
        if (kind === "dialog") {
            host.setAttribute("role", "dialog");
            host.append(document.createElement("button"));
        }
        document.body.append(host);
        const target = host.querySelector("button") ?? host;

        // when
        fireEvent.keyDown(target, { key: "ArrowRight" });
        fireEvent.keyDown(target, { key: "Escape" });

        // then
        expect(keys.onPresent).not.toHaveBeenCalled();
    });

    it("stops listening once unmounted", () => {
        // given
        const keys = handlers();
        const { unmount } = renderHook(() => usePresentationKeys(keys));

        // when
        unmount();
        fireEvent.keyDown(window, { key: "ArrowRight" });

        // then
        expect(keys.onPresent).not.toHaveBeenCalled();
    });
});
