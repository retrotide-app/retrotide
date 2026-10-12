import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { useBoardLayout } from "./useBoardLayout";

const { isNarrowViewport, subscribeNarrowViewport } = vi.hoisted(() => ({
    isNarrowViewport: vi.fn(),
    subscribeNarrowViewport: vi.fn(),
}));

vi.mock("../../platform/viewport", () => ({ isNarrowViewport, subscribeNarrowViewport }));

let notify: () => void = () => {
    throw new Error("nothing subscribed to the viewport");
};

beforeEach(() => {
    vi.clearAllMocks();
    isNarrowViewport.mockReturnValue(false);
    subscribeNarrowViewport.mockImplementation((onChange: () => void) => {
        notify = onChange;

        return vi.fn();
    });
});

describe("useBoardLayout", () => {
    it("shows columns on a wide screen", () => {
        // when
        const { result } = renderHook(() => useBoardLayout());

        // then
        expect(result.current.layout).toBe("columns");
    });

    it("shows the list on a phone-width screen", () => {
        // given
        isNarrowViewport.mockReturnValue(true);

        // when
        const { result } = renderHook(() => useBoardLayout());

        // then
        expect(result.current.layout).toBe("list");
    });

    it("follows the screen when it crosses the breakpoint", () => {
        // given
        const { result } = renderHook(() => useBoardLayout());

        // when
        isNarrowViewport.mockReturnValue(true);
        act(() => {
            notify();
        });

        // then
        expect(result.current.layout).toBe("list");
    });

    it("keeps the layout the viewer picked, whatever the screen width", () => {
        // given
        const { result } = renderHook(() => useBoardLayout());

        // when
        act(() => {
            result.current.choose("list");
        });
        isNarrowViewport.mockReturnValue(false);
        act(() => {
            notify();
        });

        // then
        expect(result.current.layout).toBe("list");
    });
});
