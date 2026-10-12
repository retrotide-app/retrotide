import { describe, expect, it } from "vitest";
import { makeBoardSnapshot, makeBoardView, makeCard, makeColumn } from "../../test-utils/fixtures";
import type { BoardSnapshot } from "../../types/api";
import { boardClipboardText, cardClipboardText, columnClipboardText } from "./clipboardText";

const START = "00000000-0000-4000-8000-000000000c01";
const STOP = "00000000-0000-4000-8000-000000000c02";

function board(overrides: Partial<BoardSnapshot> = {}): BoardSnapshot {
    return makeBoardSnapshot({
        board: makeBoardView({ title: "Sprint 12", cards_hidden: false, stage: "discuss" }),
        columns: [
            makeColumn({ id: START, title: "Start", position: "a0" }),
            makeColumn({ id: STOP, title: "Stop", position: "a1" }),
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
                body: "Long\nmeetings",
                hidden: false,
                votes: { total: 1 },
            }),
        ],
        hidden_counts: { [STOP]: 2 },
        ...overrides,
    });
}

describe("boardClipboardText", () => {
    it("writes the title, each column in order and each stack with its merged cards", () => {
        // when
        const text = boardClipboardText(board());

        // then
        expect(text).toBe(
            [
                "# Sprint 12",
                "",
                "## Start",
                "- Pair on reviews (4 votes)",
                "  - Review in pairs",
                "- Demo every Friday",
                "",
                "## Stop",
                "- Long meetings (1 vote)",
            ].join("\n"),
        );
    });

    it("leaves out hidden totals, hidden placeholders and author names", () => {
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
        const text = boardClipboardText(snapshot);

        // then
        expect(text).toBe(["# Sprint 12", "", "## Start", "- Mine, still hidden", "", "## Stop"].join("\n"));
        expect(text).not.toContain("Sam");
    });
});

describe("columnClipboardText", () => {
    it("writes one column with its heading", () => {
        // when
        const text = columnClipboardText(board(), START);

        // then
        expect(text).toBe(
            ["## Start", "- Pair on reviews (4 votes)", "  - Review in pairs", "- Demo every Friday"].join("\n"),
        );
    });

    it("writes nothing for a column the viewer cannot see", () => {
        // then
        expect(columnClipboardText(board(), "gone")).toBe("");
    });
});

describe("cardClipboardText", () => {
    it("writes a stack's top card with the cards merged under it", () => {
        // given
        const snapshot = board();

        // when
        const text = cardClipboardText(snapshot, snapshot.cards[1]);

        // then
        expect(text).toBe(["- Pair on reviews (4 votes)", "  - Review in pairs"].join("\n"));
    });

    it("writes a merged card on its own with its own total", () => {
        // given
        const snapshot = board();

        // when
        const text = cardClipboardText(snapshot, snapshot.cards[2]);

        // then
        expect(text).toBe("- Review in pairs (1 vote)");
    });
});
