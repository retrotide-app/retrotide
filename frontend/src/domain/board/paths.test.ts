import { describe, expect, it } from "vitest";
import {
    boardPath,
    boardSummaryPath,
    joinBoardPath,
    NEW_BOARD_NO_TEAM,
    newBoardPath,
    sharedBoardPath,
    sharedBoardSummaryPath,
} from "./paths";

describe("board summary paths", () => {
    it("puts the summary under the board, inside and outside the organisation", () => {
        // when
        const paths = [boardSummaryPath("harbour", "a/b"), sharedBoardSummaryPath("a/b")];

        // then
        expect(paths).toEqual(["/o/harbour/boards/a%2Fb/summary", "/boards/a%2Fb/summary"]);
    });
});

describe("sharedBoardPath", () => {
    it("opens the board outside any organisation, so a guest can reach it", () => {
        // when
        const path = sharedBoardPath("00000000-0000-4000-8000-0000000000b1");

        // then
        expect(path).toBe("/boards/00000000-0000-4000-8000-0000000000b1");
    });

    it("escapes the board ID so it cannot add a segment", () => {
        // when
        const path = sharedBoardPath("a/b");

        // then
        expect(path).toBe("/boards/a%2Fb");
    });
});

describe("joinBoardPath", () => {
    it("puts the slug under the share link root", () => {
        // when
        const path = joinBoardPath("k7Q2");

        // then
        expect(path).toBe("/b/k7Q2");
    });

    it("escapes the slug so it cannot add a segment", () => {
        // when
        const path = joinBoardPath("a/b?c");

        // then
        expect(path).toBe("/b/a%2Fb%3Fc");
    });
});

describe("boardPath", () => {
    it("puts the board under its organisation's address", () => {
        // when
        const path = boardPath("harbour", "00000000-0000-4000-8000-0000000000b1");

        // then
        expect(path).toBe("/o/harbour/boards/00000000-0000-4000-8000-0000000000b1");
    });

    it("escapes both parts so neither can add a segment", () => {
        // when
        const path = boardPath("a/b", "c?d");

        // then
        expect(path).toBe("/o/a%2Fb/boards/c%3Fd");
    });
});

describe("newBoardPath", () => {
    it("opens the wizard under the organisation's address", () => {
        // when
        const path = newBoardPath("harbour");

        // then
        expect(path).toBe("/o/harbour/boards/new");
    });

    it("names the team to create in when one is given, escaped", () => {
        // when
        const path = newBoardPath("a/b", "team&crew");

        // then
        expect(path).toBe("/o/a%2Fb/boards/new?team=team%26crew");
    });

    it("asks for a board with no team when given the no team marker", () => {
        // when
        const path = newBoardPath("harbour", NEW_BOARD_NO_TEAM);

        // then
        expect(path).toBe("/o/harbour/boards/new?team=none");
    });
});
