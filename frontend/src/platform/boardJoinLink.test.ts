import { afterEach, describe, expect, it, vi } from "vitest";
import { BOARD_JOIN_LINK_STORAGE_PREFIX, readBoardJoinLink, writeBoardJoinLink } from "./boardJoinLink";
import { setPlatformErrorReporter } from "./errorReporter";

afterEach(() => {
    sessionStorage.clear();
    vi.restoreAllMocks();
});

describe("readBoardJoinLink", () => {
    it("finds no link for a board this tab never joined through one", () => {
        // when
        const slug = readBoardJoinLink("b1");

        // then
        expect(slug).toBeNull();
    });

    it("reads back the link this tab joined the board through, and only for that board", () => {
        // given
        writeBoardJoinLink("b1", "SLUGONE");

        // when
        const mine = readBoardJoinLink("b1");
        const other = readBoardJoinLink("b2");

        // then
        expect(mine).toBe("SLUGONE");
        expect(other).toBeNull();
        expect(sessionStorage.getItem(`${BOARD_JOIN_LINK_STORAGE_PREFIX}b1`)).toBe("SLUGONE");
    });

    it("reports a storage the browser refuses and finds no link", () => {
        // given
        const reporter = vi.fn();
        setPlatformErrorReporter(reporter);
        const failure = new Error("storage blocked");
        vi.spyOn(Storage.prototype, "getItem").mockImplementation(() => {
            throw failure;
        });

        // when
        const slug = readBoardJoinLink("b1");

        // then
        expect(slug).toBeNull();
        expect(reporter).toHaveBeenCalledWith(failure, { source: "storage" });
    });
});
