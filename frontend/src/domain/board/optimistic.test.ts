import { describe, expect, it } from "vitest";
import { makeBoardSnapshot, makeCard, makeColumn, makeMe } from "../../test-utils/fixtures";
import {
    isStillPending,
    type PendingChange,
    type PendingCommand,
    withCardMoved,
    withColumnMoved,
    withPendingCommands,
    withVote,
} from "./optimistic";
import { compareByPosition } from "./sort";

const went = makeColumn({ id: "col-went", position: "a0" });
const next = makeColumn({ id: "col-next", position: "a1" });
const later = makeColumn({ id: "col-later", position: "a2" });

const first = makeCard({ id: "card-1", column_id: "col-went", position: "a0" });
const second = makeCard({ id: "card-2", column_id: "col-went", position: "a1" });
const third = makeCard({ id: "card-3", column_id: "col-next", position: "a0" });

const snapshot = makeBoardSnapshot({ columns: [went, next, later], cards: [first, second, third] });

function orderIn(board: typeof snapshot, columnId: string): string[] {
    return board.cards
        .filter(card => card.column_id === columnId)
        .toSorted(compareByPosition)
        .map(card => card.id);
}

describe("withCardMoved", () => {
    it("puts a card at the top of another column at once", () => {
        // when
        const moved = withCardMoved(snapshot, "card-2", { column_id: "col-next", after_card_id: null });

        // then
        expect(orderIn(moved, "col-next")).toEqual(["card-2", "card-3"]);
        expect(orderIn(moved, "col-went")).toEqual(["card-1"]);
    });

    it("puts a card after the one it was dropped on within its own column", () => {
        // when
        const moved = withCardMoved(snapshot, "card-1", { column_id: "col-went", after_card_id: "card-2" });

        // then
        expect(orderIn(moved, "col-went")).toEqual(["card-2", "card-1"]);
    });

    it("leaves the board as it is for a card it does not hold", () => {
        // when
        const moved = withCardMoved(snapshot, "card-missing", { column_id: "col-next", after_card_id: null });

        // then
        expect(moved).toBe(snapshot);
    });
});

describe("withColumnMoved", () => {
    it("puts a column after the one it was dropped on", () => {
        // when
        const moved = withColumnMoved(snapshot, "col-went", { after_column_id: "col-later" });

        // then
        expect(moved.columns.toSorted(compareByPosition).map(column => column.id)).toEqual([
            "col-next",
            "col-later",
            "col-went",
        ]);
    });

    it("puts a column first when it has nothing before it", () => {
        // when
        const moved = withColumnMoved(snapshot, "col-later", { after_column_id: null });

        // then
        expect(moved.columns.toSorted(compareByPosition).map(column => column.id)).toEqual([
            "col-later",
            "col-went",
            "col-next",
        ]);
    });
});

describe("isStillPending", () => {
    const change: PendingChange = {
        kind: "card-move",
        cardId: "card-1",
        drop: { column_id: "col-next", after_card_id: null },
    };

    it("holds a command while its request is still out", () => {
        // then
        expect(isStillPending({ change, answeredAt: null }, 3)).toBe(true);
    });

    it("holds an answered command until the confirmed board reaches the version it answered with", () => {
        // then
        expect(isStillPending({ change, answeredAt: 4 }, 3)).toBe(true);
        expect(isStillPending({ change, answeredAt: 4 }, 4)).toBe(false);
        expect(isStillPending({ change, answeredAt: 4 }, 5)).toBe(false);
    });
});

describe("withPendingCommands", () => {
    const confirmed = { ...snapshot, version: 3 };

    it("shows a move whose request is still out over the confirmed board", () => {
        // when
        const shown = withPendingCommands(confirmed, [
            {
                change: { kind: "card-move", cardId: "card-2", drop: { column_id: "col-next", after_card_id: null } },
                answeredAt: null,
            },
        ]);

        // then
        expect(orderIn(shown, "col-next")).toEqual(["card-2", "card-3"]);
        expect(shown.version).toBe(3);
    });

    it("keeps showing an answered move until its frame has reached the confirmed board", () => {
        // given
        const command = {
            change: { kind: "column-move", columnId: "col-later", drop: { after_column_id: null } },
            answeredAt: 4,
        } satisfies PendingCommand;

        // when
        const before = withPendingCommands(confirmed, [command]);
        const after = withPendingCommands({ ...confirmed, version: 4 }, [command]);

        // then
        expect(before.columns.toSorted(compareByPosition)[0].id).toBe("col-later");
        expect(after).toEqual({ ...confirmed, version: 4 });
    });

    it("lands in the same order once the frame has applied the move, so the card does not jump", () => {
        // given
        const drop = { column_id: "col-next", after_card_id: "card-3" };
        const command: PendingCommand = { change: { kind: "card-move", cardId: "card-1", drop }, answeredAt: null };
        const echoed = {
            ...confirmed,
            version: 4,
            cards: [{ ...first, column_id: "col-next", position: "a1" }, second, third],
        };

        // when
        const whilePending = withPendingCommands(confirmed, [command]);
        const echoedWhilePending = withPendingCommands(echoed, [command]);
        const settled = withPendingCommands(echoed, [{ ...command, answeredAt: 4 }]);

        // then
        expect(orderIn(whilePending, "col-next")).toEqual(["card-3", "card-1"]);
        expect(orderIn(echoedWhilePending, "col-next")).toEqual(["card-3", "card-1"]);
        expect(orderIn(settled, "col-next")).toEqual(["card-3", "card-1"]);
    });

    it("lays pending moves over the board in the order they were made", () => {
        // when
        const shown = withPendingCommands(confirmed, [
            {
                change: { kind: "card-move", cardId: "card-1", drop: { column_id: "col-later", after_card_id: null } },
                answeredAt: null,
            },
            {
                change: {
                    kind: "card-move",
                    cardId: "card-2",
                    drop: { column_id: "col-later", after_card_id: "card-1" },
                },
                answeredAt: null,
            },
        ]);

        // then
        expect(orderIn(shown, "col-later")).toEqual(["card-1", "card-2"]);
    });

    it("skips entries that are not optimistic commands", () => {
        // when
        const shown = withPendingCommands(confirmed, [null, null]);

        // then
        expect(shown).toBe(confirmed);
    });
});

describe("withCardMoved for stacks", () => {
    it("takes a moved stack's merged cards along and frees a moved merged card from its stack", () => {
        // given
        const top = makeCard({ id: "top", column_id: "col-went", position: "a0" });
        const under = makeCard({ id: "under", column_id: "col-went", position: "a1", parent_card_id: "top" });
        const board = makeBoardSnapshot({ columns: [went, next], cards: [top, under, third] });

        // when
        const movedTop = withCardMoved(board, "top", { column_id: "col-next", after_card_id: null });
        const movedUnder = withCardMoved(board, "under", { column_id: "col-next", after_card_id: "card-3" });

        // then
        expect(movedTop.cards.find(card => card.id === "under")).toMatchObject({
            column_id: "col-next",
            parent_card_id: "top",
        });
        expect(movedUnder.cards.find(card => card.id === "under")).toMatchObject({
            column_id: "col-next",
            parent_card_id: null,
        });
        expect(orderIn(movedTop, "col-next").filter(id => id !== "under")).toEqual(["top", "card-3"]);
    });
});

describe("withVote", () => {
    const voting = makeBoardSnapshot({
        version: 3,
        cards: [makeCard({ id: "card-1", votes: { total: 2 } })],
        me: makeMe({ votes_remaining: 5, my_votes: { "card-1": 1 } }),
    });

    it("adds a vote to the card's total and to the viewer's own votes while the request is out", () => {
        // when
        const shown = withVote(voting, "card-1", 1, null);

        // then
        expect(shown.cards[0].votes.total).toBe(3);
        expect(shown.me.my_votes).toEqual({ "card-1": 2 });
        expect(shown.me.votes_remaining).toBe(4);
    });

    it("takes a vote back, dropping the card from the viewer's votes when none are left on it", () => {
        // when
        const shown = withVote(voting, "card-1", -1, null);

        // then
        expect(shown.cards[0].votes.total).toBe(1);
        expect(shown.me.my_votes).toEqual({});
        expect(shown.me.votes_remaining).toBe(6);
    });

    it("leaves a hidden total hidden", () => {
        // given
        const hidden = { ...voting, cards: [makeCard({ id: "card-1", votes: { total: null } })] };

        // when
        const shown = withVote(hidden, "card-1", 1, null);

        // then
        expect(shown.cards[0].votes.total).toBeNull();
        expect(shown.me.my_votes).toEqual({ "card-1": 2 });
    });

    it("stops adding to the viewer's votes once the answer's votes are held, while the total still waits", () => {
        // given
        const answered = { ...voting, me: makeMe({ votes_remaining: 4, my_votes: { "card-1": 2 }, version: 4 }) };

        // when
        const shown = withVote(answered, "card-1", 1, 4);

        // then
        expect(shown.me.my_votes).toEqual({ "card-1": 2 });
        expect(shown.cards[0].votes.total).toBe(3);
    });

    it("leaves the board alone for a card it does not hold", () => {
        // then
        expect(withVote(voting, "card-missing", 1, null)).toBe(voting);
    });
});

describe("withPendingCommands for votes", () => {
    const voting = makeBoardSnapshot({
        version: 3,
        cards: [makeCard({ id: "card-1", votes: { total: 2 } })],
        me: makeMe({ votes_remaining: 5, my_votes: { "card-1": 1 } }),
    });
    const command: PendingCommand = { change: { kind: "vote", cardId: "card-1", delta: 1 }, answeredAt: 4 };

    it("drops an answered vote once both the total and the viewer's votes have caught up", () => {
        // given
        const caughtUp = {
            ...voting,
            version: 4,
            cards: [makeCard({ id: "card-1", votes: { total: 3 } })],
            me: makeMe({ votes_remaining: 4, my_votes: { "card-1": 2 }, version: 4 }),
        };

        // when
        const shown = withPendingCommands(caughtUp, [command]);

        // then
        expect(shown).toBe(caughtUp);
    });

    it("keeps the viewer's vote shown when the total has caught up but their own votes have not", () => {
        // given
        const totalOnly = {
            ...voting,
            version: 4,
            cards: [makeCard({ id: "card-1", votes: { total: 3 } })],
            me: makeMe({ votes_remaining: 5, my_votes: { "card-1": 1 }, version: 3 }),
        };

        // when
        const shown = withPendingCommands(totalOnly, [command]);

        // then
        expect(shown.cards[0].votes.total).toBe(3);
        expect(shown.me.my_votes).toEqual({ "card-1": 2 });
    });
});
