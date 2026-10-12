import { describe, expect, it } from "vitest";
import { makeBoardSnapshot, makeCard } from "../../test-utils/fixtures";
import { cardMoveChoices, dropPreview, mergeChoices, readDragData, resolveDrop } from "./drop";

const START = "00000000-0000-4000-8000-000000000c01";
const STOP = "00000000-0000-4000-8000-000000000c02";
const CONTINUE = "00000000-0000-4000-8000-000000000c03";

const first = makeCard({ id: "k1", column_id: START, position: "a0" });
const second = makeCard({ id: "k2", column_id: START, position: "a1" });
const third = makeCard({ id: "k3", column_id: START, position: "a2" });
const elsewhere = makeCard({ id: "k9", column_id: STOP, position: "a0" });

const snapshot = makeBoardSnapshot({ cards: [third, elsewhere, first, second] });

describe("resolveDrop", () => {
    it("puts a card dropped on the lower half of a card after that card", () => {
        // when
        const move = resolveDrop(snapshot, { kind: "card", id: "k1" }, { kind: "card", id: "k3" }, undefined, "after");

        // then
        expect(move).toEqual({ kind: "card", cardId: "k1", drop: { column_id: START, after_card_id: "k3" } });
    });

    it("puts a card dropped on the upper half of a card below it just before that card", () => {
        // when
        const move = resolveDrop(snapshot, { kind: "card", id: "k1" }, { kind: "card", id: "k3" }, undefined, "before");

        // then
        expect(move).toEqual({ kind: "card", cardId: "k1", drop: { column_id: START, after_card_id: "k2" } });
    });

    it("previews a drop as the line it lands on, or the card it merges into", () => {
        // when
        const line = dropPreview(
            snapshot,
            resolveDrop(snapshot, { kind: "card", id: "k1" }, { kind: "card", id: "k3" }, undefined, "after"),
        );
        const top = dropPreview(
            snapshot,
            resolveDrop(snapshot, { kind: "card", id: "k3" }, { kind: "card", id: "k1" }, undefined, "before"),
        );
        const merge = dropPreview(snapshot, { kind: "merge", cardId: "k1", targetCardId: "k3" });

        // then
        expect(line).toEqual({ kind: "line", cardId: "k3", side: "after" });
        expect(top).toEqual({ kind: "line", cardId: "k1", side: "before" });
        expect(merge).toEqual({ kind: "merge", cardId: "k3" });
    });

    it("puts a card dropped on a card above it in the same column before that card", () => {
        // when
        const move = resolveDrop(snapshot, { kind: "card", id: "k3" }, { kind: "card", id: "k2" });

        // then
        expect(move).toEqual({ kind: "card", cardId: "k3", drop: { column_id: START, after_card_id: "k1" } });
    });

    it("puts a card dropped on a card in another column before that card", () => {
        // when
        const move = resolveDrop(snapshot, { kind: "card", id: "k2" }, { kind: "card", id: "k9" });

        // then
        expect(move).toEqual({ kind: "card", cardId: "k2", drop: { column_id: STOP, after_card_id: null } });
    });

    it("sends a card dropped on a column's card area to the bottom of that column", () => {
        // when
        const move = resolveDrop(snapshot, { kind: "card", id: "k9" }, { kind: "column-cards", id: START });

        // then
        expect(move).toEqual({ kind: "card", cardId: "k9", drop: { column_id: START, after_card_id: "k3" } });
    });

    it("moves nothing when a card is dropped back where it was", () => {
        // when
        const moves = [
            resolveDrop(snapshot, { kind: "card", id: "k2" }, { kind: "card", id: "k2" }),
            resolveDrop(snapshot, { kind: "card", id: "k3" }, { kind: "column-cards", id: START }),
        ];

        // then
        expect(moves).toEqual([null, null]);
    });

    it("moves nothing when the card or the place it was dropped on has gone", () => {
        // when
        const moves = [
            resolveDrop(snapshot, { kind: "card", id: "gone" }, { kind: "column-cards", id: STOP }),
            resolveDrop(snapshot, { kind: "card", id: "k1" }, { kind: "card", id: "gone" }),
            resolveDrop(snapshot, { kind: "card", id: "k1" }, { kind: "column-cards", id: "gone" }),
        ];

        // then
        expect(moves).toEqual([null, null, null]);
    });

    it("moves a column dropped on a column to that column's place", () => {
        // when
        const moves = [
            resolveDrop(snapshot, { kind: "column", id: START }, { kind: "column", id: CONTINUE }),
            resolveDrop(snapshot, { kind: "column", id: CONTINUE }, { kind: "column", id: START }),
        ];

        // then
        expect(moves).toEqual([
            { kind: "column", columnId: START, drop: { after_column_id: CONTINUE } },
            { kind: "column", columnId: CONTINUE, drop: { after_column_id: null } },
        ]);
    });

    it("moves nothing when a column is dropped on itself or on a card", () => {
        // when
        const moves = [
            resolveDrop(snapshot, { kind: "column", id: STOP }, { kind: "column", id: STOP }),
            resolveDrop(snapshot, { kind: "column", id: STOP }, { kind: "card", id: "k1" }),
        ];

        // then
        expect(moves).toEqual([null, null]);
    });
});

describe("cardMoveChoices", () => {
    it("offers every real move for a card in the middle of its column, without repeats", () => {
        // when
        const choices = cardMoveChoices(snapshot, "k2");

        // then
        expect(choices.map(choice => choice.label)).toEqual([
            "Up one place",
            "Down one place",
            "Top of Stop",
            "Bottom of Stop",
            "Top of Continue",
        ]);
        expect(choices[0].drop).toEqual({ column_id: START, after_card_id: null });
        expect(choices[1].drop).toEqual({ column_id: START, after_card_id: "k3" });
        expect(choices[3].drop).toEqual({ column_id: STOP, after_card_id: "k9" });
    });

    it("leaves out moves that would put the card where it already is", () => {
        // when
        const choices = cardMoveChoices(snapshot, "k1");

        // then
        expect(choices.map(choice => choice.label)).toEqual([
            "Down one place",
            "Bottom of Start",
            "Top of Stop",
            "Bottom of Stop",
            "Top of Continue",
        ]);
    });

    it("offers nothing for a card that is not on the board", () => {
        // then
        expect(cardMoveChoices(snapshot, "gone")).toEqual([]);
    });
});

describe("readDragData", () => {
    it("reads the board's own drag data", () => {
        // then
        expect(readDragData({ kind: "card", id: "k1" })).toEqual({ kind: "card", id: "k1" });
    });

    it("refuses anything else", () => {
        // when
        const read = [
            readDragData(undefined),
            readDragData(null),
            readDragData({ kind: "card" }),
            readDragData({ kind: "vote", id: "k1" }),
            readDragData({ kind: "card", id: 7 }),
        ];

        // then
        expect(read).toEqual([null, null, null, null, null]);
    });

    it("reads a card's merge target", () => {
        // then
        expect(readDragData({ kind: "card-merge", id: "k1" })).toEqual({ kind: "card-merge", id: "k1" });
    });
});

describe("resolveDrop for merges", () => {
    const top = makeCard({ id: "top", column_id: STOP, position: "a1" });
    const under = makeCard({ id: "under", column_id: STOP, position: "a2", parent_card_id: "top" });
    const stacked = makeBoardSnapshot({ cards: [first, second, top, under] });
    const merging = { reorder: true, merge: true };

    it("merges a card dropped on another card's merge target", () => {
        // when
        const move = resolveDrop(stacked, { kind: "card", id: "k1" }, { kind: "card-merge", id: "k2" }, merging);

        // then
        expect(move).toEqual({ kind: "merge", cardId: "k1", targetCardId: "k2" });
    });

    it("merges into the top card when the drop lands on a merged card", () => {
        // when
        const move = resolveDrop(stacked, { kind: "card", id: "k1" }, { kind: "card-merge", id: "under" }, merging);

        // then
        expect(move).toEqual({ kind: "merge", cardId: "k1", targetCardId: "top" });
    });

    it("does nothing for a card dropped on itself or on its own stack", () => {
        // then
        expect(resolveDrop(stacked, { kind: "card", id: "k1" }, { kind: "card-merge", id: "k1" }, merging)).toBeNull();
        expect(
            resolveDrop(stacked, { kind: "card", id: "top" }, { kind: "card-merge", id: "under" }, merging),
        ).toBeNull();
    });

    it("does nothing for a merge drop while merging is not allowed", () => {
        // when
        const move = resolveDrop(stacked, { kind: "card", id: "k1" }, { kind: "card-merge", id: "k2" });

        // then
        expect(move).toBeNull();
    });

    it("refuses to reorder while the view is sorted or filtered, but still merges", () => {
        // given
        const rules = { reorder: false, merge: true };

        // then
        expect(resolveDrop(stacked, { kind: "card", id: "k1" }, { kind: "card", id: "k2" }, rules)).toBeNull();
        expect(resolveDrop(stacked, { kind: "card", id: "k1" }, { kind: "card-merge", id: "k2" }, rules)).toEqual({
            kind: "merge",
            cardId: "k1",
            targetCardId: "k2",
        });
    });
});

describe("mergeChoices", () => {
    it("offers every other top card on the board, by column, but never the card's own stack", () => {
        // given
        const top = makeCard({ id: "top", column_id: STOP, position: "a1", body: "Standups" });
        const under = makeCard({ id: "under", column_id: STOP, position: "a2", parent_card_id: "top" });
        const board = makeBoardSnapshot({
            cards: [makeCard({ id: "k1", column_id: START, position: "a0", body: "Deploys" }), top, under],
        });

        // when
        const choices = mergeChoices(board, "under");

        // then
        expect(choices).toEqual([{ cardId: "k1", label: "Start: Deploys" }]);
    });

    it("shortens a long card in its label", () => {
        // given
        const long = makeCard({ id: "k1", column_id: START, body: "x".repeat(200) });
        const board = makeBoardSnapshot({ cards: [long, makeCard({ id: "k2", column_id: STOP })] });

        // when
        const [choice] = mergeChoices(board, "k2");

        // then
        expect(choice.label).toBe(`Start: ${"x".repeat(80)}...`);
    });
});
