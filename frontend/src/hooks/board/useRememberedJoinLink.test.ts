import { renderHook } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import { rejoinState } from "../../domain/board/shareLink";
import { readBoardJoinLink, writeBoardJoinLink } from "../../platform/boardJoinLink";
import { useRememberedJoinLink } from "./useRememberedJoinLink";

afterEach(() => {
    sessionStorage.clear();
});

describe("useRememberedJoinLink", () => {
    it("uses the link the visitor just joined through and remembers it for this tab", () => {
        // when
        const { result } = renderHook(() => useRememberedJoinLink("b1", rejoinState("SLUGONE")));

        // then
        expect(result.current).toBe("SLUGONE");
        expect(readBoardJoinLink("b1")).toBe("SLUGONE");
    });

    it("finds the remembered link after a reload, when the route carries none", () => {
        // given
        writeBoardJoinLink("b1", "SLUGONE");

        // when
        const { result } = renderHook(() => useRememberedJoinLink("b1", null));

        // then
        expect(result.current).toBe("SLUGONE");
    });

    it("offers no link for a board this tab never joined through one", () => {
        // when
        const { result } = renderHook(() => useRememberedJoinLink("b2", null));

        // then
        expect(result.current).toBeNull();
    });
});
