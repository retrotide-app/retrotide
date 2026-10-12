import { describe, expect, it } from "vitest";
import {
    makeActionItem,
    makeBoardSettings,
    makeBoardSnapshot,
    makeBoardView,
    makeCard,
    makeColumn,
    makeComment,
} from "../../test-utils/fixtures";
import type { BoardSnapshot, CardView } from "../../types/api";
import {
    nextPresented,
    presentationOrder,
    presentationProgress,
    presentedStack,
    previousPresented,
} from "./presentation";

const START = "00000000-0000-4000-8000-000000000c01";
const STOP = "00000000-0000-4000-8000-000000000c02";
const CONTINUE = "00000000-0000-4000-8000-000000000c03";

const columns = [
    makeColumn({ id: CONTINUE, title: "Continue", position: "a2" }),
    makeColumn({ id: START, title: "Start", position: "a0" }),
    makeColumn({ id: STOP, title: "Stop", position: "a1" }),
];

function shownCard(id: string, column: string, position: string, total = 0, overrides: Partial<CardView> = {}) {
    return makeCard({ id, column_id: column, position, hidden: false, votes: { total }, ...overrides });
}

function shownBoard(overrides: Partial<BoardSnapshot> = {}): BoardSnapshot {
    return makeBoardSnapshot({
        board: makeBoardView({ stage: "discuss", cards_hidden: false }),
        columns,
        hidden_counts: {},
        ...overrides,
    });
}

describe("presentationOrder", () => {
    it("walks the columns in board order and each column's stack tops in position order", () => {
        // given
        const snapshot = shownBoard({
            cards: [
                shownCard("stop-1", STOP, "a0"),
                shownCard("start-2", START, "a1"),
                shownCard("start-1", START, "a0"),
                shownCard("merged", START, "a2", 0, { parent_card_id: "start-1" }),
                shownCard("continue-1", CONTINUE, "a0"),
            ],
        });

        // when
        const order = presentationOrder(snapshot);

        // then a merged card is presented with its stack, never on its own
        expect(order).toEqual(["start-1", "start-2", "stop-1", "continue-1"]);
    });

    it("follows the board's votes sort, most first, with position breaking a tie", () => {
        // given
        const snapshot = shownBoard({
            settings: makeBoardSettings({ sort_mode: "votes" }),
            cards: [
                shownCard("few", START, "a0", 1),
                shownCard("tied-later", START, "a2", 4),
                shownCard("tied-first", START, "a1", 4),
                shownCard("stacked", START, "a3", 2),
                shownCard("under", START, "a4", 3, { parent_card_id: "stacked" }),
            ],
        });

        // when
        const order = presentationOrder(snapshot);

        // then a stack counts the votes of every card in it
        expect(order).toEqual(["stacked", "tied-first", "tied-later", "few"]);
    });

    it("skips a hidden column, a column whose cards are still hidden and a card hidden from everyone else", () => {
        // given cards are hidden, and only Start has been revealed
        const snapshot = shownBoard({
            board: makeBoardView({ stage: "write", cards_hidden: true }),
            columns: [
                makeColumn({ id: START, position: "a0", cards_revealed: true }),
                makeColumn({ id: STOP, position: "a1" }),
                makeColumn({ id: CONTINUE, position: "a2", hidden: true, cards_revealed: true }),
            ],
            cards: [
                shownCard("revealed", START, "a0"),
                shownCard("mine", START, "a1", 0, { hidden: true }),
                shownCard("still-hidden", STOP, "a0", 0, { hidden: true }),
                shownCard("in-hidden-column", CONTINUE, "a0"),
            ],
        });

        // when
        const order = presentationOrder(snapshot);

        // then
        expect(order).toEqual(["revealed"]);
    });

    it("is empty while every card is hidden", () => {
        // when
        const order = presentationOrder(makeBoardSnapshot());

        // then
        expect(order).toEqual([]);
    });
});

describe("nextPresented and previousPresented", () => {
    const order = ["one", "two", "three"];

    it("step one card either way", () => {
        // when
        const next = nextPresented(order, "two");
        const previous = previousPresented(order, "two");

        // then
        expect(next).toBe("three");
        expect(previous).toBe("one");
    });

    it("stop at the ends", () => {
        // when
        const next = nextPresented(order, "three");
        const previous = previousPresented(order, "one");

        // then
        expect(next).toBeNull();
        expect(previous).toBeNull();
    });

    it("find no step from a card that is not in the order", () => {
        // when
        const next = nextPresented(order, "gone");
        const previous = previousPresented(order, "gone");

        // then
        expect(next).toBeNull();
        expect(previous).toBeNull();
    });
});

describe("presentationProgress", () => {
    it("counts the presented card's place from one", () => {
        // when
        const progress = presentationProgress(["one", "two", "three"], "two");

        // then
        expect(progress).toEqual({ position: 2, total: 3 });
    });

    it("has no place for a card outside the order", () => {
        // when
        const progress = presentationProgress(["one"], "gone");

        // then
        expect(progress).toBeNull();
    });
});

describe("presentedStack", () => {
    it("gathers the top card, its merged cards, their comments and the action items made from them", () => {
        // given
        const snapshot = shownBoard({
            cards: [
                shownCard("top", START, "a0"),
                shownCard("under", START, "a1", 0, { parent_card_id: "top" }),
                shownCard("other", START, "a2"),
            ],
            comments: [
                makeComment({ id: "on-top", card_id: "top" }),
                makeComment({ id: "on-under", card_id: "under" }),
                makeComment({ id: "on-other", card_id: "other" }),
            ],
            action_items: [
                makeActionItem({ id: "from-under", source_card_id: "under" }),
                makeActionItem({ id: "from-other", source_card_id: "other" }),
                makeActionItem({ id: "unsourced", source_card_id: null }),
            ],
        });

        // when
        const stack = presentedStack(snapshot, "under");

        // then a merged card resolves to its stack's top card
        expect(stack?.card.id).toBe("top");
        expect(stack?.merged.map(card => card.id)).toEqual(["under"]);
        expect(stack?.comments.map(comment => comment.id)).toEqual(["on-top", "on-under"]);
        expect(stack?.actionItems.map(item => item.id)).toEqual(["from-under"]);
    });

    it("finds nothing for a card the board no longer holds", () => {
        // when
        const stack = presentedStack(shownBoard(), "gone");

        // then
        expect(stack).toBeNull();
    });
});
