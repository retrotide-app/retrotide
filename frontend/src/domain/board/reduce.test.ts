import { describe, expect, it } from "vitest";
import {
    makeActionItem,
    makeBoardParticipant,
    makeBoardSettings,
    makeBoardSnapshot,
    makeBoardView,
    makeCard,
    makeColumn,
    makeComment,
    makeMe,
    makeParticipantViewer,
    makePerson,
    makePreviousAction,
    makeSpotlight,
    makeSurvey,
    makeTimer,
} from "../../test-utils/fixtures";
import type { BoardSnapshot } from "../../types/api";
import { applyBoardEvent, applyBoardEvents } from "./reduce";

const START = "00000000-0000-4000-8000-000000000c01";
const STOP = "00000000-0000-4000-8000-000000000c02";
const KEEP = "00000000-0000-4000-8000-000000000c03";

function board(overrides: Partial<BoardSnapshot> = {}): BoardSnapshot {
    return makeBoardSnapshot({
        version: 10,
        cards: [
            makeCard({ id: "k1", column_id: START, position: "a0" }),
            makeCard({ id: "k2", column_id: START, position: "a1", is_mine: undefined, author: null }),
            makeCard({ id: "k3", column_id: STOP, position: "a0" }),
        ],
        hidden_counts: { [START]: 2, [STOP]: 1 },
        ...overrides,
    });
}

describe("applyBoardEvents", () => {
    it("applies events in order and takes the frame's last version", () => {
        // given
        const held = board();

        // when
        const next = applyBoardEvents(
            held,
            [
                { type: "card_deleted", version: 11, card_id: "k1" },
                { type: "card_deleted", version: 12, card_id: "k3" },
            ],
            13,
        );

        // then
        expect(next.version).toBe(13);
        expect(next.cards.map(card => card.id)).toEqual(["k2"]);
    });

    it("moves the version on when the frame held nothing the viewer may see", () => {
        // when
        const next = applyBoardEvents(board(), [], 12);

        // then
        expect(next.version).toBe(12);
        expect(next.cards).toEqual(board().cards);
    });

    it("never changes the snapshot it was given", () => {
        // given
        const held = board();
        const copy = structuredClone(held);

        // when
        applyBoardEvents(held, [{ type: "column_deleted", version: 11, column_id: START }], 11);

        // then
        expect(held).toEqual(copy);
    });
});

describe("applyBoardEvent", () => {
    it("takes a new title and context from board_updated", () => {
        // when
        const next = applyBoardEvent(board(), {
            type: "board_updated",
            version: 11,
            title: "Sprint 13",
            context: "Release",
        });

        // then
        expect(next.board.title).toBe("Sprint 13");
        expect(next.board.context).toBe("Release");
        expect(next.board.anonymity).toBe("named");
    });

    it("adds a new participant and replaces one it already holds", () => {
        // given
        const held = board({ participants: [makeBoardParticipant({ participant_id: "p1", display_name: "Sam" })] });

        // when
        const added = applyBoardEvent(held, {
            type: "participant_upserted",
            version: 11,
            participant: { participant_id: "p2", display_name: "Ali", role: "participant" },
        });
        const renamed = applyBoardEvent(added, {
            type: "participant_upserted",
            version: 12,
            participant: { participant_id: "p1", display_name: "Samantha", role: "facilitator" },
        });

        // then
        expect(added.participants.map(participant => participant.display_name)).toEqual(["Sam", "Ali"]);
        expect(renamed.participants.map(participant => participant.display_name).sort()).toEqual(["Ali", "Samantha"]);
        expect(renamed.participants).toHaveLength(2);
    });

    it("adds a new column and replaces an edited one", () => {
        // given
        const fresh = makeColumn({ id: "c9", title: "Kudos", position: "a3" });

        // when
        const added = applyBoardEvent(board(), { type: "column_upserted", version: 11, column: fresh });
        const edited = applyBoardEvent(added, {
            type: "column_upserted",
            version: 12,
            column: { ...fresh, title: "Thanks" },
        });

        // then
        expect(added.columns).toHaveLength(4);
        expect(edited.columns).toHaveLength(4);
        expect(edited.columns.find(column => column.id === "c9")?.title).toBe("Thanks");
    });

    it("moves a column and takes every rewritten key", () => {
        // when
        const next = applyBoardEvent(board(), {
            type: "column_moved",
            version: 11,
            column_id: KEEP,
            position: "a0V",
            rewritten: [{ id: STOP, position: "a5" }],
        });

        // then
        const positions = Object.fromEntries(next.columns.map(column => [column.id, column.position]));
        expect(positions).toEqual({ [START]: "a0", [STOP]: "a5", [KEEP]: "a0V" });
    });

    it("drops a deleted column with its cards and its hidden count", () => {
        // when
        const next = applyBoardEvent(board(), { type: "column_deleted", version: 11, column_id: START });

        // then
        expect(next.columns.map(column => column.id)).toEqual([STOP, KEEP]);
        expect(next.cards.map(card => card.id)).toEqual(["k3"]);
        expect(next.hidden_counts).toEqual({ [STOP]: 1 });
    });

    it("empties a cleared column of its cards and its hidden count but keeps the column", () => {
        // when
        const next = applyBoardEvent(board(), { type: "column_cleared", version: 11, column_id: START });

        // then
        expect(next.columns).toHaveLength(3);
        expect(next.cards.map(card => card.id)).toEqual(["k3"]);
        expect(next.hidden_counts).toEqual({ [STOP]: 1 });
    });

    it("adds a new card and replaces an edited one", () => {
        // given
        const fresh = makeCard({ id: "k9", column_id: KEEP, body: "Ship it" });

        // when
        const added = applyBoardEvent(board(), { type: "card_upserted", version: 11, card: fresh });
        const edited = applyBoardEvent(added, {
            type: "card_upserted",
            version: 12,
            card: { ...fresh, body: "Ship it today" },
        });

        // then
        expect(added.cards).toHaveLength(4);
        expect(edited.cards).toHaveLength(4);
        expect(edited.cards.find(card => card.id === "k9")?.body).toBe("Ship it today");
    });

    it("keeps a card marked as the viewer's own when an event about it carries no is_mine", () => {
        // given
        const mine = board().cards[0];

        // when
        const next = applyBoardEvent(board(), {
            type: "card_upserted",
            version: 11,
            card: { ...mine, body: "Edited", is_mine: undefined },
        });

        // then
        expect(next.cards[0].is_mine).toBe(true);
        expect(next.cards[0].body).toBe("Edited");
    });

    it("takes the card's mentions from each event, so a mention taken out of the text disappears", () => {
        // given
        const sam = makePerson({ user_id: "u1", display_name: "Sam" });
        const ali = makePerson({ user_id: "u3", display_name: "Ali" });
        const named = makeCard({ id: "k9", column_id: KEEP, body: "Ask @Ali and @Sam", mentions: [ali, sam] });
        const held = applyBoardEvent(board(), { type: "card_upserted", version: 11, card: named });

        // when
        const kept = applyBoardEvent(held, {
            type: "card_upserted",
            version: 12,
            card: { ...named, body: "Ask @Sam", mentions: [sam] },
        });
        const cleared = applyBoardEvent(kept, {
            type: "card_upserted",
            version: 13,
            card: { ...named, body: "Ask nobody", mentions: undefined },
        });

        // then
        expect(kept.cards.find(card => card.id === "k9")?.mentions).toEqual([sam]);
        expect(cleared.cards.find(card => card.id === "k9")?.mentions).toBeUndefined();
    });

    it("takes a comment's mentions from each event", () => {
        // given
        const sam = makePerson({ user_id: "u1", display_name: "Sam" });
        const comment = makeComment({ id: "m9", card_id: "k1", body: "Ask @Sam", mentions: [sam] });
        const held = applyBoardEvent(board(), { type: "comment_upserted", version: 11, comment });

        // when
        const next = applyBoardEvent(held, {
            type: "comment_upserted",
            version: 12,
            comment: { ...comment, body: "Ask later", mentions: undefined },
        });

        // then
        expect(held.comments.find(posted => posted.id === "m9")?.mentions).toEqual([sam]);
        expect(next.comments.find(posted => posted.id === "m9")?.mentions).toBeUndefined();
    });

    it("never marks someone else's card as the viewer's own", () => {
        // given
        const theirs = board().cards[1];

        // when
        const next = applyBoardEvent(board(), {
            type: "card_upserted",
            version: 11,
            card: { ...theirs, body: "Edited" },
        });

        // then
        expect(next.cards[1].is_mine).toBeUndefined();
    });

    it("moves a card into another column and takes its siblings' rewritten keys", () => {
        // when
        const next = applyBoardEvent(board(), {
            type: "card_moved",
            version: 11,
            card_id: "k1",
            column_id: STOP,
            position: "a0V",
            rewritten: [{ id: "k3", position: "Zz" }],
        });

        // then
        const moved = next.cards.find(card => card.id === "k1");
        expect(moved?.column_id).toBe(STOP);
        expect(moved?.position).toBe("a0V");
        expect(next.cards.find(card => card.id === "k3")?.position).toBe("Zz");
        expect(next.cards.find(card => card.id === "k2")?.position).toBe("a1");
    });

    it("ignores rewritten keys for cards it does not hold", () => {
        // when
        const next = applyBoardEvent(board(), {
            type: "card_moved",
            version: 11,
            card_id: "k1",
            column_id: START,
            position: "a2",
            rewritten: [{ id: "unseen", position: "a3" }],
        });

        // then
        expect(next.cards).toHaveLength(3);
        expect(next.cards.some(card => card.id === "unseen")).toBe(false);
    });

    it("drops a deleted card", () => {
        // when
        const next = applyBoardEvent(board(), { type: "card_deleted", version: 11, card_id: "k2" });

        // then
        expect(next.cards.map(card => card.id)).toEqual(["k1", "k3"]);
    });

    it("replaces the whole hidden count map, so a column that reached zero disappears", () => {
        // when
        const next = applyBoardEvent(board(), { type: "hidden_counts", version: 11, hidden_counts: { [KEEP]: 4 } });

        // then
        expect(next.hidden_counts).toEqual({ [KEEP]: 4 });
    });

    it("leaves the board alone for a resync, which the sequencer turns into a refetch", () => {
        // given
        const held = board();

        // when
        const next = applyBoardEvent(held, { type: "resync", version: 11 });

        // then
        expect(next).toBe(held);
    });
});

describe("applyBoardEvent for votes and settings", () => {
    function voted(overrides: Partial<BoardSnapshot> = {}): BoardSnapshot {
        return board({
            me: makeMe({ votes_remaining: 3, my_votes: { k1: 2, k3: 1 } }),
            ...overrides,
        });
    }

    it("takes a card's new total from votes_changed", () => {
        // when
        const next = applyBoardEvent(board(), { type: "votes_changed", version: 11, card_id: "k2", total: 4 });

        // then
        expect(next.cards.find(card => card.id === "k2")?.votes.total).toBe(4);
        expect(next.cards.find(card => card.id === "k1")?.votes.total).toBe(0);
    });

    it("keeps a total hidden when votes_changed carries none", () => {
        // when
        const next = applyBoardEvent(board(), { type: "votes_changed", version: 11, card_id: "k1", total: null });

        // then
        expect(next.cards.find(card => card.id === "k1")?.votes.total).toBeNull();
    });

    it("zeroes every total and gives the viewer their whole allowance back on votes_reset", () => {
        // given
        const held = voted({
            cards: [
                makeCard({ id: "k1", column_id: START, votes: { total: 5 } }),
                makeCard({ id: "k3", column_id: STOP, votes: { total: 2 } }),
            ],
        });

        // when
        const next = applyBoardEvent(held, { type: "votes_reset", version: 11 });

        // then
        expect(next.cards.map(card => card.votes.total)).toEqual([0, 0]);
        expect(next.me).toEqual({ votes_remaining: 6, my_votes: {}, version: 11 });
    });

    it("leaves hidden totals hidden on votes_reset", () => {
        // given
        const held = board({ cards: [makeCard({ id: "k1", votes: { total: null } })] });

        // when
        const next = applyBoardEvent(held, { type: "votes_reset", version: 11 });

        // then
        expect(next.cards[0].votes.total).toBeNull();
    });

    it("ignores a reset older than the viewer's own votes, which already count it", () => {
        // given
        const held = voted({ me: makeMe({ votes_remaining: 5, my_votes: { k1: 1 }, version: 12 }) });

        // when
        const next = applyBoardEvent(held, { type: "votes_reset", version: 11 });

        // then
        expect(next.me).toEqual(held.me);
    });

    it("takes new settings and works out the viewer's remaining votes against the new limit", () => {
        // given
        const held = voted();

        // when
        const next = applyBoardEvent(held, {
            type: "settings_changed",
            version: 11,
            settings: makeBoardSettings({ max_votes: 4, voting_open: false }),
        });

        // then
        expect(next.settings.voting_open).toBe(false);
        expect(next.settings.max_votes).toBe(4);
        expect(next.me.votes_remaining).toBe(1);
        expect(next.me.my_votes).toEqual({ k1: 2, k3: 1 });
    });

    it("hides every card total the moment the facilitator hides vote totals", () => {
        // given
        const held = board({
            cards: [
                makeCard({ id: "k1", column_id: START, position: "a0", votes: { total: 3 } }),
                makeCard({ id: "k3", column_id: STOP, position: "a0", votes: { total: 1 } }),
            ],
        });

        // when
        const next = applyBoardEvent(held, {
            type: "settings_changed",
            version: 11,
            settings: makeBoardSettings({ vote_counts_hidden: true }),
        });

        // then
        expect(next.cards.map(card => card.votes.total)).toEqual([null, null]);
    });

    it("keeps card totals when settings change with totals shown", () => {
        // when
        const next = applyBoardEvent(board(), {
            type: "settings_changed",
            version: 11,
            settings: makeBoardSettings({ vote_counts_hidden: false }),
        });

        // then
        expect(next.cards).toEqual(board().cards);
    });

    it.each(["discuss", "actions"] as const)(
        "keeps every card total in %s when the facilitator hides vote totals, because totals stay shown from Discuss on",
        stage => {
            // given
            const held = board({
                board: makeBoardView({ cards_hidden: false, stage }),
                cards: [makeCard({ id: "k1", column_id: START, hidden: false, votes: { total: 3 } })],
                hidden_counts: {},
            });

            // when
            const next = applyBoardEvent(held, {
                type: "settings_changed",
                version: 11,
                settings: makeBoardSettings({ vote_counts_hidden: true }),
            });

            // then
            expect(next.settings.vote_counts_hidden).toBe(true);
            expect(next.cards.map(card => card.votes.total)).toEqual([3]);
        },
    );

    it("never takes the remaining votes below zero when the limit drops under what was spent", () => {
        // when
        const next = applyBoardEvent(voted(), {
            type: "settings_changed",
            version: 11,
            settings: makeBoardSettings({ max_votes: 1 }),
        });

        // then
        expect(next.me.votes_remaining).toBe(0);
    });

    it("drops a deleted card's comments and the viewer's votes on it, and gives those votes back", () => {
        // given
        const held = voted({
            comments: [makeComment({ id: "m1", card_id: "k1" }), makeComment({ id: "m2", card_id: "k3" })],
        });

        // when
        const next = applyBoardEvent(held, { type: "card_deleted", version: 11, card_id: "k1" });

        // then
        expect(next.comments.map(comment => comment.id)).toEqual(["m2"]);
        expect(next.me.my_votes).toEqual({ k3: 1 });
        expect(next.me.votes_remaining).toBe(5);
    });

    it("drops the viewer's votes and the comments on every card of a cleared column", () => {
        // given
        const held = voted({ comments: [makeComment({ id: "m1", card_id: "k2" })] });

        // when
        const next = applyBoardEvent(held, { type: "column_cleared", version: 11, column_id: START });

        // then
        expect(next.comments).toEqual([]);
        expect(next.me.my_votes).toEqual({ k3: 1 });
        expect(next.me.votes_remaining).toBe(5);
    });

    it("drops the viewer's votes on the cards of a deleted column", () => {
        // when
        const next = applyBoardEvent(voted(), { type: "column_deleted", version: 11, column_id: STOP });

        // then
        expect(next.me.my_votes).toEqual({ k1: 2 });
        expect(next.me.votes_remaining).toBe(4);
    });
});

describe("applyBoardEvent for comments", () => {
    it("adds a new comment and replaces an edited one, keeping the viewer's ownership", () => {
        // given
        const mine = makeComment({ id: "m1", card_id: "k1", body: "Agreed" });
        const held = board({ comments: [mine] });

        // when
        const edited = applyBoardEvent(held, {
            type: "comment_upserted",
            version: 11,
            comment: { ...mine, body: "Agreed, mostly", is_mine: undefined },
        });
        const added = applyBoardEvent(edited, {
            type: "comment_upserted",
            version: 12,
            comment: makeComment({ id: "m2", card_id: "k1", is_mine: undefined, author: null }),
        });

        // then
        expect(added.comments.map(comment => comment.body)).toEqual(["Agreed, mostly", "Agreed, reviews drag on"]);
        expect(added.comments[0].is_mine).toBe(true);
        expect(added.comments[1].is_mine).toBeUndefined();
    });

    it("drops a deleted comment", () => {
        // given
        const held = board({ comments: [makeComment({ id: "m1" }), makeComment({ id: "m2" })] });

        // when
        const next = applyBoardEvent(held, { type: "comment_deleted", version: 11, comment_id: "m1", card_id: "k1" });

        // then
        expect(next.comments.map(comment => comment.id)).toEqual(["m2"]);
    });
});

describe("applyBoardEvent for merged cards", () => {
    function stacked(): BoardSnapshot {
        return board({
            cards: [
                makeCard({ id: "top", column_id: START, position: "a0" }),
                makeCard({ id: "under", column_id: START, position: "a1", parent_card_id: "top" }),
                makeCard({ id: "other", column_id: STOP, position: "a0" }),
            ],
        });
    }

    it("stacks a merged card under its top card in the top card's column", () => {
        // when
        const next = applyBoardEvent(stacked(), {
            type: "cards_merged",
            version: 11,
            card_id: "other",
            parent_card_id: "top",
            column_id: START,
            position: "a2",
            restacked: [],
        });

        // then
        const merged = next.cards.find(card => card.id === "other");
        expect(merged).toMatchObject({ parent_card_id: "top", column_id: START, position: "a2" });
    });

    it("moves a dragged stack's own cards under the new top card", () => {
        // when
        const next = applyBoardEvent(stacked(), {
            type: "cards_merged",
            version: 11,
            card_id: "top",
            parent_card_id: "other",
            column_id: STOP,
            position: "a1",
            restacked: ["under"],
        });

        // then
        expect(next.cards.find(card => card.id === "top")).toMatchObject({ parent_card_id: "other", column_id: STOP });
        expect(next.cards.find(card => card.id === "under")).toMatchObject({
            parent_card_id: "other",
            column_id: STOP,
        });
    });

    it("takes a card out of its stack and places it as card_moved would", () => {
        // when
        const next = applyBoardEvent(stacked(), {
            type: "card_unmerged",
            version: 11,
            card_id: "under",
            column_id: START,
            position: "a0V",
            rewritten: [{ id: "other", position: "a9" }],
        });

        // then
        expect(next.cards.find(card => card.id === "under")).toMatchObject({ parent_card_id: null, position: "a0V" });
        expect(next.cards.find(card => card.id === "other")?.position).toBe("a9");
    });

    it("carries a moved top card's stack into its new column and frees a moved stacked card", () => {
        // when
        const movedTop = applyBoardEvent(stacked(), {
            type: "card_moved",
            version: 11,
            card_id: "top",
            column_id: KEEP,
            position: "a0",
            rewritten: [],
        });
        const movedUnder = applyBoardEvent(stacked(), {
            type: "card_moved",
            version: 11,
            card_id: "under",
            column_id: STOP,
            position: "a1",
            rewritten: [],
        });

        // then
        expect(movedTop.cards.find(card => card.id === "under")).toMatchObject({
            parent_card_id: "top",
            column_id: KEEP,
        });
        expect(movedUnder.cards.find(card => card.id === "under")).toMatchObject({
            parent_card_id: null,
            column_id: STOP,
        });
    });

    it("leaves a deleted top card's stacked cards on the board with no parent", () => {
        // when
        const next = applyBoardEvent(stacked(), { type: "card_deleted", version: 11, card_id: "top" });

        // then
        expect(next.cards.map(card => [card.id, card.parent_card_id])).toEqual([
            ["under", null],
            ["other", null],
        ]);
    });
});

describe("applyBoardEvents and the viewer's own votes", () => {
    it("marks the viewer's votes with the version they were read at before applying a frame", () => {
        // given
        const held = board({ me: makeMe({ votes_remaining: 4, my_votes: { k1: 2 } }) });

        // when
        const next = applyBoardEvents(held, [{ type: "votes_changed", version: 11, card_id: "k1", total: 3 }], 11);

        // then
        expect(next.me.version).toBe(10);
        expect(next.me.my_votes).toEqual({ k1: 2 });
    });
});

describe("applyBoardEvent for facilitation", () => {
    it("drops a removed participant from the list and leaves their cards on the board", () => {
        // given
        const held = board({
            participants: [
                makeBoardParticipant({ participant_id: "p1", display_name: "Sam" }),
                makeBoardParticipant({ participant_id: "p2", display_name: "Ali", role: "participant" }),
            ],
        });

        // when
        const next = applyBoardEvent(held, { type: "participant_removed", version: 11, participant_id: "p2" });

        // then
        expect(next.participants.map(participant => participant.participant_id)).toEqual(["p1"]);
        expect(next.cards).toEqual(held.cards);
    });

    it("replaces every placeholder with the revealed cards and keeps the viewer's own marked as theirs", () => {
        // given
        const held = board({
            comments: [makeComment({ id: "m1", card_id: "k1" })],
        });
        const shown = makeCard({ id: "k1", column_id: START, hidden: false, is_mine: undefined });
        const theirs = makeCard({
            id: "k9",
            column_id: STOP,
            hidden: false,
            is_mine: undefined,
            author: { participant_id: "p2", display_name: "Ali" },
        });
        const comment = makeComment({ id: "m1", card_id: "k1", is_mine: undefined });
        const reply = makeComment({ id: "m2", card_id: "k9", is_mine: undefined, author: null });

        // when
        const next = applyBoardEvent(held, {
            type: "cards_revealed",
            version: 11,
            stage: "vote",
            cards: [shown, theirs],
            comments: [comment, reply],
        });

        // then
        expect(next.board.cards_hidden).toBe(false);
        expect(next.board.stage).toBe("vote");
        expect(next.hidden_counts).toEqual({});
        expect(next.cards.map(card => [card.id, card.is_mine, card.hidden])).toEqual([
            ["k1", true, false],
            ["k9", undefined, false],
        ]);
        expect(next.comments.map(shownComment => [shownComment.id, shownComment.is_mine])).toEqual([
            ["m1", true],
            ["m2", undefined],
        ]);
    });

    it("keeps the viewer's own comment on someone else's card theirs through a re-hide and the next reveal", () => {
        // given
        const held = board({
            board: makeBoardView({ anonymity: "anonymous", cards_hidden: false }),
            cards: [makeCard({ id: "k9", column_id: STOP, hidden: false, is_mine: undefined, author: null })],
            comments: [makeComment({ id: "m2", card_id: "k9", author: null })],
            hidden_counts: {},
        });
        const rehidden = applyBoardEvent(held, { type: "cards_hidden", version: 11, hidden_counts: { [STOP]: 1 } });

        // when
        const next = applyBoardEvent(rehidden, {
            type: "cards_revealed",
            version: 12,
            stage: "write",
            cards: [makeCard({ id: "k9", column_id: STOP, hidden: false, is_mine: undefined, author: null })],
            comments: [makeComment({ id: "m2", card_id: "k9", is_mine: true, author: null })],
        });

        // then
        expect(rehidden.comments).toEqual([]);
        expect(next.comments.map(comment => [comment.id, comment.is_mine])).toEqual([["m2", true]]);
    });

    it("takes the revealed order, so a shuffled anonymous board shows its new keys", () => {
        // given
        const held = board({ board: makeBoardView({ anonymity: "anonymous" }) });

        // when
        const next = applyBoardEvent(held, {
            type: "cards_revealed",
            version: 11,
            stage: "vote",
            cards: [
                makeCard({ id: "k3", column_id: START, position: "a0", author: null, is_mine: undefined }),
                makeCard({ id: "k1", column_id: START, position: "a1", author: null, is_mine: undefined }),
            ],
            comments: [],
        });

        // then
        expect(next.cards.map(card => [card.id, card.position, card.is_mine])).toEqual([
            ["k3", "a0", true],
            ["k1", "a1", true],
        ]);
    });

    it("re-hides by keeping only the viewer's own cards, unstacked and marked hidden, and taking the new counts", () => {
        // given
        const held = board({
            board: makeBoardView({ cards_hidden: false }),
            cards: [
                makeCard({ id: "k1", column_id: START, hidden: false }),
                makeCard({ id: "k2", column_id: START, hidden: false, parent_card_id: "k1", is_mine: undefined }),
                makeCard({ id: "k3", column_id: STOP, hidden: false, parent_card_id: "k2" }),
            ],
            comments: [makeComment({ id: "m1", card_id: "k1" }), makeComment({ id: "m2", card_id: "k2" })],
            hidden_counts: {},
        });

        // when
        const next = applyBoardEvent(held, {
            type: "cards_hidden",
            version: 11,
            hidden_counts: { [START]: 1 },
        });

        // then
        expect(next.board.cards_hidden).toBe(true);
        expect(next.hidden_counts).toEqual({ [START]: 1 });
        expect(next.cards.map(card => [card.id, card.hidden, card.parent_card_id])).toEqual([
            ["k1", true, null],
            ["k3", true, null],
        ]);
        expect(next.comments.map(comment => comment.id)).toEqual(["m1"]);
    });

    it("forgets the viewer's votes on cards a re-hide dropped, but keeps them spent", () => {
        // given
        const held = board({
            board: makeBoardView({ cards_hidden: false }),
            me: makeMe({ votes_remaining: 3, my_votes: { k1: 1, k2: 2 } }),
        });

        // when
        const next = applyBoardEvents(held, [{ type: "cards_hidden", version: 11, hidden_counts: {} }], 11);

        // then
        expect(next.me.my_votes).toEqual({ k1: 1 });
        expect(next.me.votes_remaining).toBe(3);
        expect(next.me.version).toBe(11);
    });

    it("re-hides even when the viewer's counts are empty", () => {
        // given
        const held = board({ board: makeBoardView({ cards_hidden: false }), hidden_counts: {} });

        // when
        const next = applyBoardEvent(held, { type: "cards_hidden", version: 11, hidden_counts: {} });

        // then
        expect(next.board.cards_hidden).toBe(true);
        expect(next.cards.every(card => card.is_mine === true)).toBe(true);
    });

    it("takes the timer and the server's clock from timer_changed, and clears a stopped timer", () => {
        // given
        const timer = makeTimer();

        // when
        const started = applyBoardEvent(board(), {
            type: "timer_changed",
            version: 11,
            timer,
            server_now: "2026-10-01T09:31:00.000Z",
        });
        const stopped = applyBoardEvent(started, {
            type: "timer_changed",
            version: 12,
            timer: null,
            server_now: "2026-10-01T09:32:00.000Z",
        });

        // then
        expect(started.timer).toEqual(timer);
        expect(started.server_now).toBe("2026-10-01T09:31:00.000Z");
        expect(stopped.timer).toBeNull();
        expect(stopped.server_now).toBe("2026-10-01T09:32:00.000Z");
    });

    it("takes the new locks and the timer sound from settings_changed", () => {
        // when
        const next = applyBoardEvent(board(), {
            type: "settings_changed",
            version: 11,
            settings: makeBoardSettings({ cards_locked: true, moves_locked: true, timer_play_sound: false }),
        });

        // then
        expect(next.settings).toMatchObject({ cards_locked: true, moves_locked: true, timer_play_sound: false });
    });
});

describe("applyBoardEvent for stages", () => {
    it("moves the board to the stage stage_changed names and leaves its cards alone", () => {
        // given
        const held = board({ board: makeBoardView({ cards_hidden: false, stage: "vote" }), hidden_counts: {} });

        // when
        const next = applyBoardEvent(held, { type: "stage_changed", version: 11, stage: "discuss" });

        // then
        expect(next.board.stage).toBe("discuss");
        expect(next.board.cards_hidden).toBe(false);
        expect(next.cards).toEqual(held.cards);
        expect(next.comments).toEqual(held.comments);
    });

    it("keeps the board in Write when the settings page reveals the cards there", () => {
        // given
        const held = board();

        // when
        const next = applyBoardEvent(held, {
            type: "cards_revealed",
            version: 11,
            stage: "write",
            cards: [makeCard({ id: "k1", column_id: START, hidden: false })],
            comments: [],
        });

        // then
        expect(next.board.stage).toBe("write");
        expect(next.board.cards_hidden).toBe(false);
        expect(next.hidden_counts).toEqual({});
    });

    it("takes the stage at the frame's version, so a later frame moves it on again", () => {
        // given
        const held = board({ board: makeBoardView({ cards_hidden: false, stage: "vote" }), hidden_counts: {} });

        // when
        const next = applyBoardEvents(
            held,
            [
                { type: "stage_changed", version: 11, stage: "discuss" },
                { type: "stage_changed", version: 12, stage: "actions" },
            ],
            12,
        );

        // then
        expect(next.board.stage).toBe("actions");
        expect(next.version).toBe(12);
    });

    it("steps the board back a stage and keeps the cards shown and the votes cast", () => {
        // given
        const held = board({
            board: makeBoardView({ cards_hidden: false, stage: "discuss" }),
            hidden_counts: {},
            me: makeMe({ my_votes: { k1: 1 }, votes_remaining: 5 }),
        });

        // when
        const next = applyBoardEvent(held, { type: "stage_changed", version: 11, stage: "vote" });

        // then
        expect(next.board.stage).toBe("vote");
        expect(next.board.cards_hidden).toBe(false);
        expect(next.cards).toEqual(held.cards);
        expect(next.me).toEqual(held.me);
    });
});

describe("applyBoardEvent for action items", () => {
    const first = makeActionItem({ id: "i1", title: "Book the room" });
    const second = makeActionItem({ id: "i2", title: "Write the notes" });

    it("adds a new action item after the ones the board holds, keeping them oldest first", () => {
        // given
        const held = board({ action_items: [first] });

        // when
        const next = applyBoardEvent(held, { type: "action_item_upserted", version: 11, action_item: second });

        // then
        expect(next.action_items).toEqual([first, second]);
    });

    it("replaces an action item in place when it is edited or marked", () => {
        // given
        const held = board({ action_items: [first, second] });
        const done = { ...first, status: "done" as const, assignee: makePerson() };

        // when
        const next = applyBoardEvent(held, { type: "action_item_upserted", version: 11, action_item: done });

        // then
        expect(next.action_items).toEqual([done, second]);
    });

    it("keeps the is_mine the server sends the item's creator", () => {
        // given
        const held = board({ action_items: [first] });
        const own = { ...first, title: "Book the big room", is_mine: true as const };

        // when
        const next = applyBoardEvent(held, { type: "action_item_upserted", version: 11, action_item: own });

        // then
        expect(next.action_items[0].is_mine).toBe(true);
    });

    it("drops a removed action item", () => {
        // given
        const held = board({ action_items: [first, second] });

        // when
        const next = applyBoardEvent(held, { type: "action_item_removed", version: 11, action_item_id: "i1" });

        // then
        expect(next.action_items).toEqual([second]);
    });

    it("takes a reviewed previous action in place, with its outcome", () => {
        // given
        const earlier = makePreviousAction({ id: "p1" });
        const held = board({ previous_actions: [earlier] });
        const reviewed = { ...earlier, status: "done" as const, review: "done" as const };

        // when
        const next = applyBoardEvent(held, {
            type: "previous_action_upserted",
            version: 11,
            previous_action: reviewed,
        });

        // then
        expect(next.previous_actions).toEqual([reviewed]);
        expect(next.action_items).toEqual(held.action_items);
    });

    it("leaves the cards alone when an action item changes", () => {
        // given
        const held = board({ action_items: [first] });

        // when
        const next = applyBoardEvent(held, { type: "action_item_removed", version: 11, action_item_id: "i1" });

        // then
        expect(next.cards).toBe(held.cards);
        expect(next.previous_actions).toBe(held.previous_actions);
    });
});

describe("applyBoardEvent for column facilitation", () => {
    function participantBoard(overrides: Partial<BoardSnapshot> = {}): BoardSnapshot {
        return board({ viewer: makeParticipantViewer(), ...overrides });
    }

    it("replaces a revealed column's placeholders with its cards, keeps the viewer's own and drops its count", () => {
        // given a board hiding cards, holding only the viewer's own card in Start
        const held = board({
            cards: [makeCard({ id: "k1", column_id: START }), makeCard({ id: "k3", column_id: STOP })],
            comments: [makeComment({ id: "m1", card_id: "k1" }), makeComment({ id: "m3", card_id: "k3" })],
        });
        const revealed = [
            makeCard({ id: "k1", column_id: START, hidden: false, is_mine: undefined }),
            makeCard({ id: "k2", column_id: START, hidden: false, is_mine: undefined, author: null }),
        ];

        // when
        const next = applyBoardEvent(held, {
            type: "column_revealed",
            version: 11,
            column_id: START,
            cards: revealed,
            comments: [makeComment({ id: "m2", card_id: "k2", is_mine: undefined })],
        });

        // then
        expect(next.cards.map(card => [card.id, card.hidden, card.is_mine ?? null])).toEqual([
            ["k3", true, true],
            ["k1", false, true],
            ["k2", false, null],
        ]);
        expect(next.comments.map(comment => comment.id)).toEqual(["m3", "m2"]);
        expect(next.hidden_counts).toEqual({ [STOP]: 1 });
        expect(next.columns.find(column => column.id === START)?.cards_revealed).toBe(true);
        expect(next.columns.find(column => column.id === STOP)?.cards_revealed).toBe(false);
    });

    it("marks a hidden column for a facilitator and keeps everything in it", () => {
        // given
        const held = board();

        // when
        const next = applyBoardEvent(held, { type: "column_hidden", version: 11, column_id: STOP });

        // then
        expect(next.columns.find(column => column.id === STOP)?.hidden).toBe(true);
        expect(next.cards).toBe(held.cards);
        expect(next.hidden_counts).toEqual(held.hidden_counts);
    });

    it("drops a hidden column for a participant with its cards, comments, count and votes, which stay spent", () => {
        // given
        const held = participantBoard({
            comments: [makeComment({ id: "m3", card_id: "k3" }), makeComment({ id: "m1", card_id: "k1" })],
            me: makeMe({ votes_remaining: 3, my_votes: { k1: 2, k3: 1 } }),
        });

        // when
        const next = applyBoardEvent(held, { type: "column_hidden", version: 11, column_id: STOP });

        // then
        expect(next.columns.map(column => column.id)).toEqual([START, KEEP]);
        expect(next.cards.map(card => card.id)).toEqual(["k1", "k2"]);
        expect(next.comments.map(comment => comment.id)).toEqual(["m1"]);
        expect(next.hidden_counts).toEqual({ [START]: 2 });
        expect(next.me.my_votes).toEqual({ k1: 2 });
        expect(next.me.votes_remaining).toBe(3);
    });

    it("zeroes one column's totals and refunds the viewer's votes on it on column_votes_reset", () => {
        // given
        const held = board({
            board: makeBoardView({ cards_hidden: false, stage: "vote" }),
            cards: [
                makeCard({ id: "k1", column_id: START, votes: { total: 3 } }),
                makeCard({ id: "k3", column_id: STOP, votes: { total: 2 } }),
            ],
            me: makeMe({ votes_remaining: 3, my_votes: { k1: 2, k3: 1 } }),
        });

        // when
        const next = applyBoardEvent(held, { type: "column_votes_reset", version: 11, column_id: START });

        // then
        expect(next.cards.map(card => [card.id, card.votes.total])).toEqual([
            ["k1", 0],
            ["k3", 2],
        ]);
        expect(next.me.my_votes).toEqual({ k3: 1 });
        expect(next.me.votes_remaining).toBe(5);
    });

    it("leaves hidden totals hidden on column_votes_reset", () => {
        // given
        const held = board({ cards: [makeCard({ id: "k1", column_id: START, votes: { total: null } })] });

        // when
        const next = applyBoardEvent(held, { type: "column_votes_reset", version: 11, column_id: START });

        // then
        expect(next.cards[0].votes.total).toBeNull();
    });

    it("forgets every per-column reveal when the whole board's cards are hidden or revealed", () => {
        // given Start revealed on its own while the board hides cards
        const revealedStart = (snapshot: BoardSnapshot) =>
            snapshot.columns.map(column => (column.id === START ? { ...column, cards_revealed: true } : column));
        const hiding = board();
        const showing = board({ board: makeBoardView({ cards_hidden: false }) });

        // when
        const rehidden = applyBoardEvent(
            { ...showing, columns: revealedStart(showing) },
            { type: "cards_hidden", version: 11, hidden_counts: {} },
        );
        const revealed = applyBoardEvent(
            { ...hiding, columns: revealedStart(hiding) },
            { type: "cards_revealed", version: 11, stage: "write", cards: [], comments: [] },
        );

        // then
        expect(rehidden.columns.map(column => column.cards_revealed)).toEqual([false, false, false]);
        expect(revealed.columns.map(column => column.cards_revealed)).toEqual([false, false, false]);
    });
});

describe("applyBoardEvent for the spotlight", () => {
    const lit = makeSpotlight({ presenting_card_id: "k1", highlighted_card_id: "k3", focused_column_id: STOP });

    it("takes the whole spotlight from spotlight_changed", () => {
        // when
        const next = applyBoardEvent(board(), { type: "spotlight_changed", version: 11, spotlight: lit });

        // then
        expect(next.spotlight).toEqual(lit);
    });

    it("ends the spotlight on a deleted card", () => {
        // when
        const next = applyBoardEvent(board({ spotlight: lit }), { type: "card_deleted", version: 11, card_id: "k1" });

        // then
        expect(next.spotlight).toEqual({ ...lit, presenting_card_id: null });
    });

    it("ends the spotlight on a deleted column and on every card it held", () => {
        // when
        const next = applyBoardEvent(board({ spotlight: lit }), {
            type: "column_deleted",
            version: 11,
            column_id: STOP,
        });

        // then
        expect(next.spotlight).toEqual({
            presenting_card_id: "k1",
            highlighted_card_id: null,
            focused_column_id: null,
        });
    });

    it("keeps a cleared column in focus but ends the spotlight on its cards", () => {
        // when
        const next = applyBoardEvent(board({ spotlight: lit }), {
            type: "column_cleared",
            version: 11,
            column_id: START,
        });

        // then
        expect(next.spotlight).toEqual({ ...lit, presenting_card_id: null });
    });

    it("ends the presentation and the highlight when cards are hidden again, and keeps the focus", () => {
        // given
        const held = board({ board: makeBoardView({ cards_hidden: false }), spotlight: lit });

        // when
        const next = applyBoardEvent(held, { type: "cards_hidden", version: 11, hidden_counts: {} });

        // then
        expect(next.spotlight).toEqual({
            presenting_card_id: null,
            highlighted_card_id: null,
            focused_column_id: STOP,
        });
    });
});

describe("applyBoardEvent for card polish", () => {
    const reacted = makeCard({
        id: "k1",
        column_id: START,
        hidden: false,
        reactions: [{ reaction: "heart", count: 2 }],
        my_reactions: ["heart"],
    });

    it("takes the counts and the viewer's own keys from card_reactions_changed", () => {
        // given
        const held = board();

        // when
        const next = applyBoardEvent(held, {
            type: "card_reactions_changed",
            version: 11,
            card_id: "k1",
            reactions: [
                { reaction: "thumbs_up", count: 1 },
                { reaction: "fire", count: 3 },
            ],
            my_reactions: ["fire"],
        });

        // then
        expect(next.cards[0].reactions).toEqual([
            { reaction: "thumbs_up", count: 1 },
            { reaction: "fire", count: 3 },
        ]);
        expect(next.cards[0].my_reactions).toEqual(["fire"]);
        expect(next.cards[1]).toBe(held.cards[1]);
    });

    it("drops the fields when the reactions run out", () => {
        // given
        const held = board({ cards: [reacted] });

        // when
        const next = applyBoardEvent(held, {
            type: "card_reactions_changed",
            version: 11,
            card_id: "k1",
            reactions: [],
            my_reactions: [],
        });

        // then
        expect(next.cards[0]).not.toHaveProperty("reactions");
        expect(next.cards[0]).not.toHaveProperty("my_reactions");
    });

    it("keeps the held reactions when an upserted or revealed card carries none", () => {
        // given
        const held = board({ cards: [reacted] });
        const bare = makeCard({ id: "k1", column_id: START, hidden: false, body: "Edited" });

        // when
        const upserted = applyBoardEvent(held, { type: "card_upserted", version: 11, card: bare });
        const revealed = applyBoardEvent(held, {
            type: "cards_revealed",
            version: 11,
            stage: "vote",
            cards: [bare],
            comments: [],
        });
        const columnRevealed = applyBoardEvent(held, {
            type: "column_revealed",
            version: 11,
            column_id: START,
            cards: [bare],
            comments: [],
        });

        // then
        for (const next of [upserted, revealed, columnRevealed]) {
            const card = next.cards.find(kept => kept.id === "k1");
            expect(card?.body).toBe("Edited");
            expect(card?.reactions).toEqual([{ reaction: "heart", count: 2 }]);
            expect(card?.my_reactions).toEqual(["heart"]);
        }
    });

    it("takes the counts of a revealed card and keeps the viewer's own keys the shared payload leaves out", () => {
        // given
        const held = board({ cards: [reacted] });
        const counted = makeCard({
            id: "k1",
            column_id: START,
            hidden: false,
            reactions: [{ reaction: "heart", count: 3 }],
        });

        // when
        const next = applyBoardEvent(held, {
            type: "cards_revealed",
            version: 11,
            stage: "vote",
            cards: [counted],
            comments: [],
        });

        // then
        expect(next.cards[0].reactions).toEqual([{ reaction: "heart", count: 3 }]);
        expect(next.cards[0].my_reactions).toEqual(["heart"]);
    });

    it("pins and unpins a card from card_pinned", () => {
        // given
        const held = board();

        // when
        const pinned = applyBoardEvent(held, { type: "card_pinned", version: 11, card_id: "k3", pinned: true });
        const unpinned = applyBoardEvent(pinned, { type: "card_pinned", version: 12, card_id: "k3", pinned: false });

        // then
        expect(pinned.cards[2].pinned).toBe(true);
        expect(unpinned.cards[2]).not.toHaveProperty("pinned");
    });

    it("sets and clears a card's image from card_image_changed", () => {
        // given
        const held = board();
        const path = "/uploads/b1/0a1b.webp";

        // when
        const attached = applyBoardEvent(held, {
            type: "card_image_changed",
            version: 11,
            card_id: "k1",
            image_path: path,
        });
        const detached = applyBoardEvent(attached, {
            type: "card_image_changed",
            version: 12,
            card_id: "k1",
            image_path: null,
        });

        // then
        expect(attached.cards[0].image_path).toBe(path);
        expect(detached.cards[0]).not.toHaveProperty("image_path");
    });

    it("sets and clears the board background from board_background_changed", () => {
        // given
        const held = board();
        const path = "/uploads/b1/1c2d.webp";

        // when
        const set = applyBoardEvent(held, { type: "board_background_changed", version: 11, background_path: path });
        const cleared = applyBoardEvent(set, { type: "board_background_changed", version: 12, background_path: null });

        // then
        expect(set.board.background_path).toBe(path);
        expect(cleared.board).not.toHaveProperty("background_path");
        expect(cleared.board.title).toBe(held.board.title);
    });
});

describe("applyBoardEvent for surveys", () => {
    it("adds a new survey and replaces a held one from survey_upserted", () => {
        // given
        const first = makeSurvey();
        const held = board({ surveys: [first] });
        const second = makeSurvey({ id: "s2", title: "Mood" });

        // when
        const added = applyBoardEvent(held, { type: "survey_upserted", version: 11, survey: second });
        const closed = applyBoardEvent(added, {
            type: "survey_upserted",
            version: 12,
            survey: { ...first, status: "closed", respondents: 4 },
        });

        // then
        expect(added.surveys.map(survey => survey.id)).toEqual([first.id, "s2"]);
        expect(closed.surveys[0].status).toBe("closed");
        expect(closed.surveys[0].respondents).toBe(4);
        expect(closed.surveys[1]).toBe(second);
    });

    it("drops a survey on survey_removed", () => {
        // given
        const held = board({ surveys: [makeSurvey(), makeSurvey({ id: "s2" })] });

        // when
        const next = applyBoardEvent(held, { type: "survey_removed", version: 11, survey_id: "s2" });

        // then
        expect(next.surveys.map(survey => survey.id)).toEqual([makeSurvey().id]);
    });
});
