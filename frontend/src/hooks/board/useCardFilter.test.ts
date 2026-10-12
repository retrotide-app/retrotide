import { act, renderHook } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import type { SortMode } from "../../types/api";
import { useCardFilter } from "./useCardFilter";

interface FilterProps {
    boardSort: SortMode;
    enforced: boolean;
}

describe("useCardFilter", () => {
    it("starts from the board's own order with nothing filtered", () => {
        // when
        const { result } = renderHook(() => useCardFilter("votes", false));

        // then
        expect(result.current.filter).toEqual({ sort: "votes", query: "", authorId: null, seed: 0 });
    });

    it("follows the facilitator's order until the viewer picks one of their own", () => {
        // given
        const { result, rerender } = renderHook((p: FilterProps) => useCardFilter(p.boardSort, p.enforced), {
            initialProps: { boardSort: "manual", enforced: false },
        });

        // when
        rerender({ boardSort: "votes", enforced: false });
        const followed = result.current.filter.sort;
        act(() => {
            result.current.sortBy("date");
        });
        rerender({ boardSort: "manual", enforced: false });

        // then
        expect(followed).toBe("votes");
        expect(result.current.filter.sort).toBe("date");
    });

    it("pins the board's order over the viewer's own while it is enforced, and follows its changes", () => {
        // given a viewer who picked their own order
        const { result, rerender } = renderHook((p: FilterProps) => useCardFilter(p.boardSort, p.enforced), {
            initialProps: { boardSort: "manual", enforced: false },
        });
        act(() => {
            result.current.sortBy("date");
        });

        // when
        rerender({ boardSort: "manual", enforced: true });
        const pinned = result.current.filter.sort;
        rerender({ boardSort: "votes", enforced: true });

        // then
        expect(pinned).toBe("manual");
        expect(result.current.filter.sort).toBe("votes");
    });

    it("ignores a sort the viewer asks for while the order is enforced", () => {
        // given
        const { result } = renderHook(() => useCardFilter("votes", true));

        // when
        act(() => {
            result.current.sortBy("random");
        });

        // then
        expect(result.current.filter).toMatchObject({ sort: "votes", seed: 0 });
    });

    it("keeps search and the author filter working while the order is enforced", () => {
        // given
        const { result } = renderHook(() => useCardFilter("votes", true));

        // when
        act(() => {
            result.current.search("deploy");
            result.current.showAuthor("p-sam");
        });

        // then
        expect(result.current.filter).toEqual({ sort: "votes", query: "deploy", authorId: "p-sam", seed: 0 });
    });

    it("keeps the search text and the author the viewer chose", () => {
        // given
        const { result } = renderHook(() => useCardFilter("manual", false));

        // when
        act(() => {
            result.current.search("deploy");
            result.current.showAuthor("p-sam");
        });

        // then
        expect(result.current.filter).toMatchObject({ query: "deploy", authorId: "p-sam" });
    });

    it("picks a new shuffle each time the viewer asks for a random order", () => {
        // given
        const { result } = renderHook(() => useCardFilter("manual", false));

        // when
        act(() => {
            result.current.sortBy("random");
        });

        // then
        expect(result.current.filter.sort).toBe("random");
        expect(Number.isInteger(result.current.filter.seed)).toBe(true);
    });
});
