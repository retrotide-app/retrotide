import { describe, expect, it } from "vitest";
import { makeBoardSettings, makeBoardSnapshot, makeCard, makeColumn, makeMe } from "../../test-utils/fixtures";
import {
    columnVoteLimitFrom,
    columnVotesLeft,
    columnVotesUsed,
    heldVoteCard,
    meAfterSettings,
    meVersion,
    meWithoutCards,
    meWithoutVotesOn,
    spentVotes,
    stackVotes,
    stampMe,
    votesLeftOnCard,
    withMe,
} from "./votes";

describe("meVersion and stampMe", () => {
    it("reads the viewer's votes as of the snapshot's version until they are stamped", () => {
        // given
        const snapshot = makeBoardSnapshot({ version: 7 });

        // when
        const stamped = stampMe(snapshot);

        // then
        expect(meVersion(snapshot)).toBe(7);
        expect(stamped.me.version).toBe(7);
        expect(stampMe(stamped)).toBe(stamped);
    });
});

describe("withMe", () => {
    it("takes the viewer's votes from a newer board_me or command answer", () => {
        // given
        const snapshot = makeBoardSnapshot({ version: 7 });

        // when
        const next = withMe(snapshot, makeMe({ votes_remaining: 5, my_votes: { k1: 1 } }), 9);

        // then
        expect(next.me).toEqual({ votes_remaining: 5, my_votes: { k1: 1 }, version: 9 });
    });

    it("keeps the votes it holds when the answer is no newer than them", () => {
        // given
        const snapshot = makeBoardSnapshot({ version: 7, me: makeMe({ votes_remaining: 4, version: 9 }) });

        // when
        const next = withMe(snapshot, makeMe({ votes_remaining: 5 }), 8);

        // then
        expect(next).toBe(snapshot);
    });
});

describe("spentVotes", () => {
    it("counts votes on cards the viewer cannot see from the allowance the server reported", () => {
        // then
        expect(spentVotes(makeMe({ votes_remaining: 2, my_votes: { k1: 1 } }), 6)).toBe(4);
        expect(spentVotes(makeMe({ votes_remaining: 0, my_votes: { k1: 7 } }), 6)).toBe(7);
    });
});

describe("meAfterSettings", () => {
    it("keeps what was spent and measures it against the new limit", () => {
        // given
        const me = makeMe({ votes_remaining: 2, my_votes: { k1: 4 } });

        // when
        const raised = meAfterSettings(me, makeBoardSettings({ max_votes: 6 }), makeBoardSettings({ max_votes: 10 }));
        const lowered = meAfterSettings(me, makeBoardSettings({ max_votes: 6 }), makeBoardSettings({ max_votes: 3 }));

        // then
        expect(raised.votes_remaining).toBe(6);
        expect(lowered.votes_remaining).toBe(0);
    });
});

describe("meWithoutCards", () => {
    it("drops the votes on removed cards and gives them back", () => {
        // given
        const me = makeMe({ votes_remaining: 1, my_votes: { k1: 3, k2: 2 } });

        // when
        const next = meWithoutCards(me, makeBoardSettings({ max_votes: 6 }), new Set(["k1"]));

        // then
        expect(next).toEqual({ votes_remaining: 4, my_votes: { k2: 2 } });
    });

    it("returns the same votes when none were on the removed cards", () => {
        // given
        const me = makeMe({ my_votes: { k2: 2 } });

        // when
        const next = meWithoutCards(me, makeBoardSettings(), new Set(["k1"]));

        // then
        expect(next).toBe(me);
    });
});

describe("stackVotes", () => {
    it("adds up a stack's totals and the viewer's votes on any card in it", () => {
        // given
        const stack = [
            makeCard({ id: "k1", votes: { total: 3 } }),
            makeCard({ id: "k2", parent_card_id: "k1", votes: { total: 2 } }),
        ];

        // when
        const votes = stackVotes(stack, makeMe({ my_votes: { k1: 1, k2: 1, k9: 4 } }));

        // then
        expect(votes).toEqual({ total: 5, mine: 2 });
    });

    it("shows no total while totals are hidden, but still the viewer's own votes", () => {
        // given
        const stack = [makeCard({ id: "k1", votes: { total: null } })];

        // when
        const votes = stackVotes(stack, makeMe({ my_votes: { k1: 2 } }));

        // then
        expect(votes).toEqual({ total: null, mine: 2 });
    });
});

describe("heldVoteCard", () => {
    it("names the card in the stack that holds the viewer's vote, the top card first", () => {
        // given
        const stack = [makeCard({ id: "k1" }), makeCard({ id: "k2", parent_card_id: "k1" })];

        // then
        expect(heldVoteCard(stack, makeMe({ my_votes: { k1: 1, k2: 1 } }))).toBe("k1");
        expect(heldVoteCard(stack, makeMe({ my_votes: { k2: 1 } }))).toBe("k2");
        expect(heldVoteCard(stack, makeMe())).toBeNull();
    });
});

describe("votesLeftOnCard", () => {
    it("is the viewer's remaining votes when cards have no limit of their own", () => {
        // given
        const snapshot = makeBoardSnapshot({ me: makeMe({ votes_remaining: 4 }) });

        // then
        expect(votesLeftOnCard(snapshot, makeCard({ id: "k1" }))).toBe(4);
    });

    it("stops at the per-card limit, counting the votes already on that card", () => {
        // given
        const snapshot = makeBoardSnapshot({
            settings: makeBoardSettings({ max_votes_per_card: 2 }),
            me: makeMe({ votes_remaining: 4, my_votes: { k1: 1, k2: 2 } }),
        });

        // then
        expect(votesLeftOnCard(snapshot, makeCard({ id: "k1" }))).toBe(1);
        expect(votesLeftOnCard(snapshot, makeCard({ id: "k2" }))).toBe(0);
    });
});

describe("meWithoutVotesOn", () => {
    it("forgets the votes on cards the viewer can no longer see, and keeps them spent", () => {
        // given
        const me = makeMe({ votes_remaining: 2, my_votes: { k1: 1, k2: 3 } });

        // when
        const next = meWithoutVotesOn(me, new Set(["k2"]));

        // then
        expect(next).toEqual({ votes_remaining: 2, my_votes: { k1: 1 } });
    });

    it("hands back the same votes when none were on a dropped card", () => {
        // given
        const me = makeMe({ my_votes: { k1: 1 } });

        // then
        expect(meWithoutVotesOn(me, new Set(["k9"]))).toBe(me);
    });
});

describe("columnVotesUsed and columnVotesLeft", () => {
    const START = "00000000-0000-4000-8000-000000000c01";
    const STOP = "00000000-0000-4000-8000-000000000c02";

    function votingSnapshot(maxVotes: number | null) {
        return makeBoardSnapshot({
            columns: [makeColumn({ id: START, max_votes: maxVotes }), makeColumn({ id: STOP, title: "Stop" })],
            cards: [
                makeCard({ id: "k1", column_id: START }),
                makeCard({ id: "k2", column_id: START }),
                makeCard({ id: "k3", column_id: STOP }),
            ],
            me: makeMe({ votes_remaining: 2, my_votes: { k1: 1, k2: 2, k3: 1 } }),
        });
    }

    it("counts only the viewer's votes on cards in that column", () => {
        // given
        const snapshot = votingSnapshot(4);

        // then
        expect(columnVotesUsed(snapshot, START)).toBe(3);
        expect(columnVotesUsed(snapshot, STOP)).toBe(1);
    });

    it("works out the votes left in a column with a limit, never below zero", () => {
        // then
        expect(columnVotesLeft(votingSnapshot(4), START)).toBe(1);
        expect(columnVotesLeft(votingSnapshot(2), START)).toBe(0);
    });

    it("has no answer for a column without a limit of its own, or one that is not on the board", () => {
        // given
        const snapshot = votingSnapshot(null);

        // then
        expect(columnVotesLeft(snapshot, START)).toBeNull();
        expect(columnVotesLeft(snapshot, "gone")).toBeNull();
    });
});

describe("columnVoteLimitFrom", () => {
    it("reads a whole number from 1 to 99", () => {
        // then
        expect(columnVoteLimitFrom(" 3 ")).toBe(3);
        expect(columnVoteLimitFrom("99")).toBe(99);
    });

    it("refuses anything else", () => {
        // then
        expect(columnVoteLimitFrom("0")).toBeNull();
        expect(columnVoteLimitFrom("100")).toBeNull();
        expect(columnVoteLimitFrom("2.5")).toBeNull();
        expect(columnVoteLimitFrom("")).toBeNull();
    });
});
