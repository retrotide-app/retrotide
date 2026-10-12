import { describe, expect, it } from "vitest";
import { makeCard } from "../../test-utils/fixtures";
import { cardStacks, stackCards, stackOf, stackTopId, topCards } from "./stacks";

const top = makeCard({ id: "top", position: "a0" });
const under = makeCard({ id: "under", position: "a2", parent_card_id: "top" });
const later = makeCard({ id: "later", position: "a1", parent_card_id: "top" });
const alone = makeCard({ id: "alone", position: "a3" });
const orphan = makeCard({ id: "orphan", position: "a4", parent_card_id: "gone" });

const cards = [under, alone, top, orphan, later];

describe("topCards", () => {
    it("keeps the cards that head a stack, and a card whose top card the viewer does not hold", () => {
        // when
        const tops = topCards(cards);

        // then
        expect(tops.map(card => card.id)).toEqual(["alone", "top", "orphan"]);
    });
});

describe("cardStacks", () => {
    it("groups each stack under its top card, both in position order", () => {
        // when
        const stacks = cardStacks(cards);

        // then
        expect(stacks.map(stack => [stack.card.id, stack.merged.map(card => card.id)])).toEqual([
            ["top", ["later", "under"]],
            ["alone", []],
            ["orphan", []],
        ]);
    });
});

describe("stackCards", () => {
    it("lists the top card first, then its merged cards", () => {
        // when
        const [stack] = cardStacks(cards);

        // then
        expect(stackCards(stack).map(card => card.id)).toEqual(["top", "later", "under"]);
    });
});

describe("stackTopId", () => {
    it("names the top card of the stack a card belongs to", () => {
        // then
        expect(stackTopId(cards, "under")).toBe("top");
        expect(stackTopId(cards, "top")).toBe("top");
        expect(stackTopId(cards, "orphan")).toBe("orphan");
        expect(stackTopId(cards, "missing")).toBeNull();
    });
});

describe("stackOf", () => {
    it("finds the whole stack from any card in it", () => {
        // when
        const stack = stackOf(cards, "later");

        // then
        expect(stack?.card.id).toBe("top");
        expect(stack?.merged.map(card => card.id)).toEqual(["later", "under"]);
        expect(stackOf(cards, "missing")).toBeNull();
    });
});
