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
    makeParticipantViewer,
} from "../../test-utils/fixtures";
import type { CardView } from "../../types/api";
import { boardSummary, hiddenCardsLabel, participantsLabel, votesLabel } from "./summary";

const START = "00000000-0000-4000-8000-000000000c01";
const STOP = "00000000-0000-4000-8000-000000000c02";

const revealed = makeBoardView({ stage: "discuss", cards_hidden: false });

const robin = makeBoardParticipant({ participant_id: "p2", display_name: "Robin" });

function card(id: string, position: string, votes: number | null, overrides: Partial<CardView> = {}): CardView {
    return makeCard({ id, position, column_id: START, hidden: false, votes: { total: votes }, ...overrides });
}

describe("boardSummary", () => {
    it("heads the summary with the board's title, stage, creation date and participant count", () => {
        // given
        const snapshot = makeBoardSnapshot({
            board: revealed,
            participants: [makeBoardParticipant(), robin],
        });

        // when
        const summary = boardSummary(snapshot);

        // then
        expect(summary).toMatchObject({
            title: "Sprint 12",
            stage: "Discuss",
            ended: false,
            createdAt: "2026-10-01T09:00:00Z",
            participants: 2,
            exportable: true,
        });
    });

    it("says an ended board ended, and carries no date when the board has none", () => {
        // given
        const snapshot = makeBoardSnapshot({
            board: makeBoardView({ closed: true, stage: "end", anonymity: "anonymous", created_at: undefined }),
        });

        // when
        const summary = boardSummary(snapshot);

        // then
        expect(summary.stage).toBe("Ended");
        expect(summary.ended).toBe(true);
        expect(summary.createdAt).toBeNull();
    });

    it("is not exportable without board.export", () => {
        // given
        const snapshot = makeBoardSnapshot({ viewer: makeParticipantViewer({ capabilities: ["board.view"] }) });

        // when
        const summary = boardSummary(snapshot);

        // then
        expect(summary.exportable).toBe(false);
    });

    it("keeps the columns in board order and sorts each column's cards by votes, most first, then by position", () => {
        // given
        const snapshot = makeBoardSnapshot({
            board: revealed,
            hidden_counts: {},
            cards: [card("k1", "a0", 1), card("k2", "a1", 4), card("k3", "a2", 1), card("k4", "a3", 2)],
        });

        // when
        const summary = boardSummary(snapshot);

        // then
        expect(summary.columns.map(column => column.column.title)).toEqual(["Start", "Stop", "Continue"]);
        expect(summary.columns[0].stacks.map(stack => stack.top.card.id)).toEqual(["k2", "k4", "k1", "k3"]);
        expect(summary.columns[0].stacks.map(stack => stack.votes)).toEqual([4, 2, 1, 1]);
    });

    it("keeps board order when vote totals are hidden", () => {
        // given
        const snapshot = makeBoardSnapshot({
            board: makeBoardView({ stage: "vote", cards_hidden: false }),
            settings: makeBoardSettings({ vote_counts_hidden: true }),
            hidden_counts: {},
            cards: [card("k1", "a0", null), card("k2", "a1", null)],
        });

        // when
        const summary = boardSummary(snapshot);

        // then
        expect(summary.columns[0].stacks.map(stack => stack.top.card.id)).toEqual(["k1", "k2"]);
        expect(summary.columns[0].stacks.map(stack => stack.votes)).toEqual([null, null]);
    });

    it("groups merged cards under their parent and ranks the group by its combined votes", () => {
        // given
        const snapshot = makeBoardSnapshot({
            board: revealed,
            hidden_counts: {},
            cards: [
                card("k1", "a0", 3),
                card("k2", "a1", 1),
                card("k3", "a2", 2, { parent_card_id: "k2" }),
                card("k4", "a3", 1, { parent_card_id: "k2" }),
            ],
        });

        // when
        const summary = boardSummary(snapshot);

        // then
        const [first, second] = summary.columns[0].stacks;
        expect(summary.columns[0].stacks).toHaveLength(2);
        expect(first.top.card.id).toBe("k2");
        expect(first.votes).toBe(4);
        expect(first.merged.map(merged => merged.card.id)).toEqual(["k3", "k4"]);
        expect(first.merged.map(merged => merged.votes)).toEqual([2, 1]);
        expect(second.top.card.id).toBe("k1");
    });

    it("puts each card's comments and action items under it", () => {
        // given
        const snapshot = makeBoardSnapshot({
            board: revealed,
            hidden_counts: {},
            cards: [card("k1", "a0", 0), card("k2", "a1", 0, { parent_card_id: "k1" })],
            comments: [
                makeComment({ id: "m1", card_id: "k1", body: "Agreed" }),
                makeComment({ id: "m2", card_id: "k2", body: "Same here" }),
            ],
            action_items: [
                makeActionItem({ id: "i1", title: "Book the room", source_card_id: "k1" }),
                makeActionItem({ id: "i2", title: "Ask @Sam", source_card_id: "k2" }),
                makeActionItem({ id: "i3", title: "Write it up", source_card_id: "k1" }),
            ],
        });

        // when
        const summary = boardSummary(snapshot);

        // then
        const [stack] = summary.columns[0].stacks;
        expect(stack.top.comments.map(comment => comment.id)).toEqual(["m1"]);
        expect(stack.top.actionItems.map(item => item.id)).toEqual(["i1", "i3"]);
        expect(stack.merged[0].comments.map(comment => comment.id)).toEqual(["m2"]);
        expect(stack.merged[0].actionItems.map(item => item.id)).toEqual(["i2"]);
        expect(summary.otherActions).toEqual([]);
    });

    it("lists actions with no source card, or a source card the viewer cannot see, as other actions", () => {
        // given
        const snapshot = makeBoardSnapshot({
            board: makeBoardView({ stage: "actions" }),
            cards: [card("k1", "a0", 0)],
            hidden_counts: { [STOP]: 1 },
            action_items: [
                makeActionItem({ id: "i1", title: "Legacy action", source_card_id: null }),
                makeActionItem({ id: "i2", title: "From a hidden card", source_card_id: "hidden-card" }),
                makeActionItem({ id: "i3", title: "From a visible card", source_card_id: "k1" }),
            ],
        });

        // when
        const summary = boardSummary(snapshot);

        // then
        expect(summary.otherActions.map(item => item.id)).toEqual(["i1", "i2"]);
        expect(summary.columns[0].stacks[0].top.actionItems.map(item => item.id)).toEqual(["i3"]);
    });

    it("counts each column's hidden cards without any of their content", () => {
        // given
        const snapshot = makeBoardSnapshot({
            columns: [makeColumn(), makeColumn({ id: STOP, title: "Stop", position: "a1" })],
            hidden_counts: { [STOP]: 2 },
        });

        // when
        const summary = boardSummary(snapshot);

        // then
        expect(summary.columns.map(column => column.hidden)).toEqual([0, 2]);
        expect(summary.columns[1].stacks).toEqual([]);
        expect(summary.columns[0].stacks[0].top.card.body).toBe("Pair on reviews");
    });
});

describe("summary labels", () => {
    it("counts in the singular for one and the plural otherwise", () => {
        // when
        const labels = [
            votesLabel(1),
            votesLabel(0),
            participantsLabel(1),
            participantsLabel(4),
            hiddenCardsLabel(1),
            hiddenCardsLabel(3),
        ];

        // then
        expect(labels).toEqual([
            "1 vote",
            "0 votes",
            "1 participant",
            "4 participants",
            "1 card from others, hidden for now",
            "3 cards from others, hidden for now",
        ]);
    });
});
