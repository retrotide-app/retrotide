import { describe, expect, it } from "vitest";
import { makeBoardSnapshot, makeBoardView, makeCard, makeColumn } from "../../test-utils/fixtures";
import type { BoardSnapshot } from "../../types/api";
import { BOARD_PRINT_ROUTE, boardImageFileName, boardPrintPath, printAction, printColumns } from "./print";

const START = "00000000-0000-4000-8000-000000000d01";
const STOP = "00000000-0000-4000-8000-000000000d02";

function board(overrides: Partial<BoardSnapshot> = {}): BoardSnapshot {
    return makeBoardSnapshot({
        board: makeBoardView({ title: "Sprint 12", cards_hidden: false, stage: "discuss" }),
        columns: [
            makeColumn({ id: STOP, title: "Stop", position: "a1" }),
            makeColumn({ id: START, title: "Start", position: "a0" }),
        ],
        cards: [
            makeCard({
                id: "k2",
                column_id: START,
                position: "a1",
                body: "Demo every Friday",
                hidden: false,
                votes: { total: 0 },
            }),
            makeCard({
                id: "k1",
                column_id: START,
                position: "a0",
                body: "Pair on reviews",
                hidden: false,
                votes: { total: 3 },
                reactions: [
                    { reaction: "heart", count: 2 },
                    { reaction: "rocket", count: 1 },
                ],
                image_path: "/uploads/b1/m1.webp",
            }),
            makeCard({
                id: "k3",
                column_id: START,
                position: "a2",
                parent_card_id: "k1",
                body: "Review in pairs",
                hidden: false,
                votes: { total: 1 },
            }),
            makeCard({
                id: "k4",
                column_id: STOP,
                position: "a0",
                body: "Long meetings",
                hidden: false,
                votes: { total: 1 },
            }),
            makeCard({
                id: "k5",
                column_id: STOP,
                position: "a1",
                body: "Late deploys",
                hidden: false,
                votes: { total: 2 },
                pinned: true,
            }),
        ],
        hidden_counts: { [STOP]: 2 },
        ...overrides,
    });
}

describe("boardPrintPath", () => {
    it("names the board and what to do once the page has drawn", () => {
        // then
        expect(boardPrintPath("b/1", "print")).toBe("/boards/b%2F1/print?then=print");
        expect(boardPrintPath("b1", "png")).toBe("/boards/b1/print?then=png");
    });

    it("sits under the shared board route", () => {
        // then
        expect(BOARD_PRINT_ROUTE).toBe("/boards/:boardId/print");
    });
});

describe("printAction", () => {
    it.each([
        { search: "?then=print", want: "print" },
        { search: "?then=png", want: "png" },
        { search: "then=png&x=1", want: "png" },
        { search: "", want: null },
        { search: "?then=pdf", want: null },
        { search: "?then=", want: null },
    ])("reads $search as $want", ({ search, want }) => {
        // when
        const action = printAction(search);

        // then
        expect(action).toBe(want);
    });
});

describe("boardImageFileName", () => {
    it("names the file like the server's exports, dated in UTC", () => {
        // given
        const date = new Date("2026-10-12T23:30:00Z");

        // when
        const name = boardImageFileName("Sprint 12", date);

        // then
        expect(name).toBe("RetroTide - Sprint 12 - 2026-10-12.png");
    });

    it("replaces the characters a file name cannot hold and squeezes the spaces", () => {
        // when
        const name = boardImageFileName('Q3: "Wins"\tand  losses / risks?', new Date("2026-01-05T08:00:00Z"));

        // then
        expect(name).toBe("RetroTide - Q3- -Wins- and losses - risks- - 2026-01-05.png");
    });
});

describe("printColumns", () => {
    it("lays the columns out in order with each stack, its votes, reactions, image and merged cards", () => {
        // when
        const columns = printColumns(board());

        // then
        expect(columns.map(({ column }) => column.title)).toEqual(["Start", "Stop"]);
        expect(columns[0]?.cards).toEqual([
            {
                id: "k1",
                body: "Pair on reviews",
                votes: 4,
                pinned: false,
                imagePath: "/uploads/b1/m1.webp",
                reactions: [
                    { key: "heart", emoji: "❤️", label: "Love", count: 2 },
                    { key: "rocket", emoji: "🚀", label: "Rocket", count: 1 },
                ],
                merged: [{ id: "k3", body: "Review in pairs" }],
            },
            {
                id: "k2",
                body: "Demo every Friday",
                votes: 0,
                pinned: false,
                imagePath: null,
                reactions: [],
                merged: [],
            },
        ]);
        expect(columns[0]?.hidden).toBe(0);
    });

    it("puts a pinned card first and counts the cards still hidden from the viewer", () => {
        // when
        const stop = printColumns(board())[1];

        // then
        expect(stop?.cards.map(card => card.id)).toEqual(["k5", "k4"]);
        expect(stop?.cards[0]?.pinned).toBe(true);
        expect(stop?.hidden).toBe(2);
    });

    it("leaves a hidden total unknown rather than guessing it", () => {
        // given
        const snapshot = board({
            cards: [
                makeCard({
                    id: "k1",
                    column_id: START,
                    body: "Mine, still hidden",
                    hidden: true,
                    votes: { total: null },
                }),
            ],
        });

        // when
        const start = printColumns(snapshot)[0];

        // then
        expect(start?.cards[0]?.votes).toBeNull();
    });
});
