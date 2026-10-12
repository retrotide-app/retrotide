import { describe, expect, it } from "vitest";
import { makeCard, makeColumn } from "../../test-utils/fixtures";
import { cardDropMoves, cardDropTarget, columnDropMoves, columnDropTarget } from "./dropTarget";

const START = "00000000-0000-4000-8000-000000000c01";
const STOP = "00000000-0000-4000-8000-000000000c02";

const first = makeCard({ id: "k1", column_id: START, position: "a0" });
const second = makeCard({ id: "k2", column_id: START, position: "a1" });
const third = makeCard({ id: "k3", column_id: START, position: "a2" });
const startCards = [third, first, second];

const startColumn = makeColumn({ id: "c1", position: "a0" });
const stopColumn = makeColumn({ id: "c2", position: "a1" });
const continueColumn = makeColumn({ id: "c3", position: "a2" });
const columns = [continueColumn, startColumn, stopColumn];

describe("cardDropTarget", () => {
    it("drops a card at the top of a column with no anchor", () => {
        // when
        const drop = cardDropTarget(startCards, "k9", START, 0);

        // then
        expect(drop).toEqual({ column_id: START, after_card_id: null });
    });

    it("anchors a drop after the card just above the slot, in position order", () => {
        // when
        const drop = cardDropTarget(startCards, "k9", START, 2);

        // then
        expect(drop).toEqual({ column_id: START, after_card_id: "k2" });
    });

    it("counts slots with the dragged card taken out of its own column", () => {
        // when
        const drop = cardDropTarget(startCards, "k1", START, 1);

        // then
        expect(drop).toEqual({ column_id: START, after_card_id: "k2" });
    });

    it("puts a slot past the end after the last card, so Move to can send a card to the bottom", () => {
        // when
        const drop = cardDropTarget(startCards, "k9", START, Number.POSITIVE_INFINITY);

        // then
        expect(drop).toEqual({ column_id: START, after_card_id: "k3" });
    });

    it("treats a slot before the start as the top", () => {
        // when
        const drop = cardDropTarget(startCards, "k9", START, -2);

        // then
        expect(drop).toEqual({ column_id: START, after_card_id: null });
    });

    it("drops into an empty column at the top", () => {
        // when
        const drop = cardDropTarget([], "k1", STOP, 3);

        // then
        expect(drop).toEqual({ column_id: STOP, after_card_id: null });
    });
});

describe("columnDropTarget", () => {
    it("moves a column to the front with no anchor", () => {
        // when
        const drop = columnDropTarget(columns, "c3", 0);

        // then
        expect(drop).toEqual({ after_column_id: null });
    });

    it("anchors a column after the column just before the slot, with itself taken out", () => {
        // when
        const drop = columnDropTarget(columns, "c1", 1);

        // then
        expect(drop).toEqual({ after_column_id: "c2" });
    });

    it("puts a slot past the end after the last column", () => {
        // when
        const drop = columnDropTarget(columns, "c1", 10);

        // then
        expect(drop).toEqual({ after_column_id: "c3" });
    });
});

describe("cardDropMoves", () => {
    it("says a drop back into the card's own place moves nothing", () => {
        // when
        const moves = [
            cardDropMoves(startCards, first, { column_id: START, after_card_id: null }),
            cardDropMoves(startCards, second, { column_id: START, after_card_id: "k1" }),
        ];

        // then
        expect(moves).toEqual([false, false]);
    });

    it("says a drop elsewhere in the same column moves the card", () => {
        // when
        const moves = cardDropMoves(startCards, first, { column_id: START, after_card_id: "k2" });

        // then
        expect(moves).toBe(true);
    });

    it("says a drop into another column moves the card", () => {
        // when
        const moves = cardDropMoves(startCards, first, { column_id: STOP, after_card_id: null });

        // then
        expect(moves).toBe(true);
    });
});

describe("columnDropMoves", () => {
    it("says a drop back into the column's own place moves nothing", () => {
        // when
        const moves = [
            columnDropMoves(columns, "c1", { after_column_id: null }),
            columnDropMoves(columns, "c3", { after_column_id: "c2" }),
        ];

        // then
        expect(moves).toEqual([false, false]);
    });

    it("says a drop anywhere else moves the column", () => {
        // when
        const moves = columnDropMoves(columns, "c1", { after_column_id: "c3" });

        // then
        expect(moves).toBe(true);
    });
});
