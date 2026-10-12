import { describe, expect, it } from "vitest";
import { makeBoardSnapshot, makeCard, makeColumn } from "../../test-utils/fixtures";
import { boardColumns, cardsInColumn, hiddenCount, hiddenPlaceholders } from "./selectors";

const START = "00000000-0000-4000-8000-000000000c01";
const STOP = "00000000-0000-4000-8000-000000000c02";
const CONTINUE = "00000000-0000-4000-8000-000000000c03";

describe("cardsInColumn", () => {
    it("keeps only the column's cards, in position order", () => {
        // given
        const cards = [
            makeCard({ id: "k3", column_id: START, position: "a2" }),
            makeCard({ id: "k1", column_id: START, position: "a0" }),
            makeCard({ id: "k9", column_id: STOP, position: "a1" }),
            makeCard({ id: "k2", column_id: START, position: "a1" }),
        ];

        // when
        const inStart = cardsInColumn(cards, START);

        // then
        expect(inStart.map(card => card.id)).toEqual(["k1", "k2", "k3"]);
    });

    it("leaves out cards merged under another card, which never anchor a move", () => {
        // given
        const cards = [
            makeCard({ id: "k1", column_id: START, position: "a0" }),
            makeCard({ id: "k2", column_id: START, position: "a1", parent_card_id: "k1" }),
        ];

        // when
        const inStart = cardsInColumn(cards, START);

        // then
        expect(inStart.map(card => card.id)).toEqual(["k1"]);
    });

    it("leaves the cards it was given in their order", () => {
        // given
        const cards = [makeCard({ id: "k2", position: "a1" }), makeCard({ id: "k1", position: "a0" })];

        // when
        cardsInColumn(cards, START);

        // then
        expect(cards.map(card => card.id)).toEqual(["k2", "k1"]);
    });
});

describe("hiddenCount", () => {
    it("reads the column's count of other people's hidden cards", () => {
        // when
        const count = hiddenCount({ [STOP]: 3 }, STOP);

        // then
        expect(count).toBe(3);
    });

    it("counts nothing for a column the counts leave out", () => {
        // when
        const count = hiddenCount({ [STOP]: 3 }, START);

        // then
        expect(count).toBe(0);
    });
});

describe("hiddenPlaceholders", () => {
    it("gives one placeholder per hidden card, keyed by column and place alone", () => {
        // when
        const placeholders = hiddenPlaceholders({ [STOP]: 3 }, STOP);

        // then
        expect(placeholders).toEqual([
            { key: `${STOP}:hidden:0` },
            { key: `${STOP}:hidden:1` },
            { key: `${STOP}:hidden:2` },
        ]);
    });

    it("gives none for a column with no hidden cards", () => {
        // when
        const placeholders = hiddenPlaceholders({}, START);

        // then
        expect(placeholders).toEqual([]);
    });
});

describe("boardColumns", () => {
    it("lays out the columns in position order, each with its cards and placeholders", () => {
        // given
        const mine = makeCard({ id: "k1", column_id: STOP });
        const snapshot = makeBoardSnapshot({
            columns: [
                makeColumn({ id: CONTINUE, title: "Continue", position: "a2" }),
                makeColumn({ id: START, title: "Start", position: "a0" }),
                makeColumn({ id: STOP, title: "Stop", position: "a1" }),
            ],
            cards: [mine],
            hidden_counts: { [STOP]: 2, [CONTINUE]: 1 },
        });

        // when
        const columns = boardColumns(snapshot);

        // then
        expect(columns.map(({ column }) => column.title)).toEqual(["Start", "Stop", "Continue"]);
        expect(columns.map(({ cards }) => cards)).toEqual([[], [mine], []]);
        expect(columns.map(({ placeholders }) => placeholders.length)).toEqual([0, 2, 1]);
        expect(columns.map(({ total }) => total)).toEqual([0, 3, 1]);
    });

    it("breaks a tie on position by column id", () => {
        // given
        const snapshot = makeBoardSnapshot({
            columns: [
                makeColumn({ id: STOP, title: "Stop", position: "a0" }),
                makeColumn({ id: START, title: "Start", position: "a0" }),
            ],
            cards: [],
            hidden_counts: {},
        });

        // when
        const columns = boardColumns(snapshot);

        // then
        expect(columns.map(({ column }) => column.title)).toEqual(["Start", "Stop"]);
    });

    it("shows a merged card inside its stack rather than on its own, and counts it", () => {
        // given
        const top = makeCard({ id: "k1", column_id: START, position: "a0" });
        const merged = makeCard({ id: "k2", column_id: START, position: "a1", parent_card_id: "k1" });
        const snapshot = makeBoardSnapshot({ cards: [merged, top], hidden_counts: {} });

        // when
        const [start] = boardColumns(snapshot);

        // then
        expect(start.cards).toEqual([top]);
        expect(start.stacks).toEqual([{ card: top, merged: [merged] }]);
        expect(start.total).toBe(2);
    });

    it("keeps only the stacks that match the search, in the chosen order, and drops the placeholders", () => {
        // given
        const snapshot = makeBoardSnapshot({
            cards: [
                makeCard({ id: "k1", column_id: STOP, position: "a0", body: "Deploy on Fridays", votes: { total: 1 } }),
                makeCard({ id: "k2", column_id: STOP, position: "a1", body: "Long standups", votes: { total: 0 } }),
                makeCard({ id: "k3", column_id: STOP, position: "a2", body: "Deploy freeze", votes: { total: 5 } }),
            ],
            hidden_counts: { [STOP]: 2 },
        });

        // when
        const [, stop] = boardColumns(snapshot, { sort: "votes", query: "deploy", authorId: null, seed: 0 });

        // then
        expect(stop.cards.map(card => card.id)).toEqual(["k3", "k1"]);
        expect(stop.placeholders).toEqual([]);
        expect(stop.total).toBe(2);
        expect(stop.filtered).toBe(true);
    });

    it("puts no text, id or author of a hidden card into a placeholder", () => {
        // given
        const snapshot = makeBoardSnapshot({ cards: [], hidden_counts: { [STOP]: 1 } });

        // when
        const [, stop] = boardColumns(snapshot);

        // then
        expect(stop.placeholders).toEqual([{ key: `${STOP}:hidden:0` }]);
    });
});
