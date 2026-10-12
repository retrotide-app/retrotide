import { describe, expect, it } from "vitest";
import { makeBoardSnapshot, makeBoardView, makeCard, makeMe } from "../../test-utils/fixtures";
import {
    authorChoices,
    authorFilterOf,
    type CardFilter,
    cardSortFrom,
    isFiltering,
    keepsBoardOrder,
    shownSort,
    sortChoices,
    sortInForce,
    sortStacks,
    stackMatches,
    UNFILTERED,
} from "./cardFilter";
import type { CardStack } from "./stacks";

const named = makeBoardView();
const anonymous = makeBoardView({ anonymity: "anonymous" });

const sam = { participant_id: "p-sam", display_name: "Sam" };
const ali = { participant_id: "p-ali", display_name: "Ali" };

function stack(
    id: string,
    overrides: Parameters<typeof makeCard>[0] = {},
    merged: CardStack["merged"] = [],
): CardStack {
    return { card: makeCard({ id, ...overrides }), merged };
}

function filter(overrides: Partial<CardFilter> = {}): CardFilter {
    return { ...UNFILTERED, ...overrides };
}

function ids(stacks: readonly CardStack[]): string[] {
    return stacks.map(entry => entry.card.id);
}

describe("sortChoices", () => {
    it("offers every order on a named board", () => {
        // then
        expect(sortChoices(named).map(choice => choice.value)).toEqual(["manual", "votes", "date", "random"]);
    });

    it("leaves out the date order on an anonymous board, whose cards carry no dates", () => {
        // then
        expect(sortChoices(anonymous).map(choice => choice.value)).toEqual(["manual", "votes", "random"]);
    });
});

describe("cardSortFrom", () => {
    it("reads only an order the board offers", () => {
        // then
        expect(cardSortFrom("votes", named)).toBe("votes");
        expect(cardSortFrom("date", anonymous)).toBeUndefined();
        expect(cardSortFrom("alphabet", named)).toBeUndefined();
    });
});

describe("sortInForce", () => {
    it("follows the board's order until the viewer picks one of their own", () => {
        // then
        expect(sortInForce(null, "votes", false)).toBe("votes");
        expect(sortInForce("date", "votes", false)).toBe("date");
    });

    it("holds everyone to the board's order while the facilitator enforces it", () => {
        // then
        expect(sortInForce("date", "votes", true)).toBe("votes");
        expect(sortInForce("random", "manual", true)).toBe("manual");
        expect(sortInForce(null, "manual", true)).toBe("manual");
    });
});

describe("shownSort and authorFilterOf", () => {
    it("falls back to board order for a date sort on an anonymous board", () => {
        // then
        expect(shownSort(filter({ sort: "date" }), anonymous)).toBe("manual");
        expect(shownSort(filter({ sort: "date" }), named)).toBe("date");
    });

    it("never filters by author on an anonymous board", () => {
        // then
        expect(authorFilterOf(filter({ authorId: sam.participant_id }), anonymous)).toBeNull();
        expect(authorFilterOf(filter({ authorId: sam.participant_id }), named)).toBe(sam.participant_id);
    });
});

describe("isFiltering and keepsBoardOrder", () => {
    it("filters only for a search with text in it or an author on a named board", () => {
        // then
        expect(isFiltering(filter({ query: "   " }), named)).toBe(false);
        expect(isFiltering(filter({ query: "deploy" }), named)).toBe(true);
        expect(isFiltering(filter({ authorId: sam.participant_id }), named)).toBe(true);
        expect(isFiltering(filter({ authorId: sam.participant_id }), anonymous)).toBe(false);
    });

    it("keeps the board's own order only when nothing is filtered or sorted", () => {
        // then
        expect(keepsBoardOrder(UNFILTERED, named)).toBe(true);
        expect(keepsBoardOrder(filter({ sort: "votes" }), named)).toBe(false);
        expect(keepsBoardOrder(filter({ query: "x" }), named)).toBe(false);
    });
});

describe("authorChoices", () => {
    it("lists each author once, by name", () => {
        // given
        const snapshot = makeBoardSnapshot({
            cards: [
                makeCard({ id: "k1", author: sam }),
                makeCard({ id: "k2", author: ali }),
                makeCard({ id: "k3", author: sam }),
                makeCard({ id: "k4", author: null }),
            ],
        });

        // then
        expect(authorChoices(snapshot)).toEqual([ali, sam]);
    });

    it("lists nobody on an anonymous board", () => {
        // given
        const snapshot = makeBoardSnapshot({ board: anonymous, cards: [makeCard({ author: sam })] });

        // then
        expect(authorChoices(snapshot)).toEqual([]);
    });
});

describe("stackMatches", () => {
    it("matches text in any card of the stack, ignoring case", () => {
        // given
        const entry = stack("k1", { body: "Deploys were smooth" }, [
            makeCard({ id: "k2", parent_card_id: "k1", body: "Standups ran long" }),
        ]);

        // then
        expect(stackMatches(entry, filter({ query: "DEPLOY" }), named)).toBe(true);
        expect(stackMatches(entry, filter({ query: "standups" }), named)).toBe(true);
        expect(stackMatches(entry, filter({ query: "retro" }), named)).toBe(false);
    });

    it("matches an author who wrote any card of the stack", () => {
        // given
        const entry = stack("k1", { author: sam }, [makeCard({ id: "k2", parent_card_id: "k1", author: ali })]);

        // then
        expect(stackMatches(entry, filter({ authorId: ali.participant_id }), named)).toBe(true);
        expect(stackMatches(entry, filter({ authorId: "p-nobody" }), named)).toBe(false);
    });

    it("ignores an author filter on an anonymous board", () => {
        // given
        const entry = stack("k1", { author: null });

        // then
        expect(stackMatches(entry, filter({ authorId: sam.participant_id }), anonymous)).toBe(true);
    });
});

describe("sortStacks", () => {
    const me = makeMe();
    const first = stack("k1", { votes: { total: 1 }, created_at: "2026-10-01T09:01:00Z" });
    const second = stack("k2", { votes: { total: 4 }, created_at: "2026-10-01T09:03:00Z" });
    const third = stack("k3", { votes: { total: 1 }, created_at: "2026-10-01T09:02:00Z" }, [
        makeCard({ id: "k4", parent_card_id: "k3", votes: { total: 2 } }),
    ]);
    const stacks = [first, second, third];

    it("keeps board order for a manual sort", () => {
        // then
        expect(ids(sortStacks(stacks, UNFILTERED, named, me))).toEqual(["k1", "k2", "k3"]);
    });

    it("puts the stacks with the most votes first, counting merged cards, and keeps board order on a tie", () => {
        // when
        const sorted = sortStacks(stacks, filter({ sort: "votes" }), named, me);

        // then
        expect(ids(sorted)).toEqual(["k2", "k3", "k1"]);
    });

    it("keeps board order for a vote sort while totals are hidden", () => {
        // given
        const hidden = stacks.map(entry => ({ ...entry, card: { ...entry.card, votes: { total: null } } }));

        // when
        const sorted = sortStacks(hidden, filter({ sort: "votes" }), named, me);

        // then
        expect(ids(sorted)).toEqual(["k1", "k2", "k3"]);
    });

    it("puts the newest cards first for a date sort", () => {
        // then
        expect(ids(sortStacks(stacks, filter({ sort: "date" }), named, me))).toEqual(["k2", "k3", "k1"]);
    });

    it("puts pinned stacks first whatever the sort, each group keeping its usual order", () => {
        // given k3 and k1 are pinned
        const pinned = [
            { ...first, card: { ...first.card, pinned: true as const } },
            second,
            { ...third, card: { ...third.card, pinned: true as const } },
        ];

        // when
        const manual = sortStacks(pinned, UNFILTERED, named, me);
        const byVotes = sortStacks(pinned, filter({ sort: "votes" }), named, me);
        const byDate = sortStacks(pinned, filter({ sort: "date" }), named, me);

        // then
        expect(ids(manual)).toEqual(["k1", "k3", "k2"]);
        expect(ids(byVotes)).toEqual(["k3", "k1", "k2"]);
        expect(ids(byDate)).toEqual(["k3", "k1", "k2"]);
    });

    it("treats a stack as pinned only when its top card is", () => {
        // given a pinned card merged under an unpinned one
        const under = stack("k5", {}, [makeCard({ id: "k6", parent_card_id: "k5", pinned: true })]);

        // when
        const sorted = sortStacks([first, under], UNFILTERED, named, me);

        // then
        expect(ids(sorted)).toEqual(["k1", "k5"]);
    });

    it("shuffles the same way for the same seed and keeps every stack", () => {
        // given
        const many = Array.from({ length: 12 }, (_, index) => stack(`k${index}`));

        // when
        const once = sortStacks(many, filter({ sort: "random", seed: 42 }), named, me);
        const again = sortStacks(many, filter({ sort: "random", seed: 42 }), named, me);
        const other = sortStacks(many, filter({ sort: "random", seed: 7 }), named, me);

        // then
        expect(ids(once)).toEqual(ids(again));
        expect(ids(once).toSorted()).toEqual(ids(many).toSorted());
        expect(ids(other)).not.toEqual(ids(once));
    });
});
