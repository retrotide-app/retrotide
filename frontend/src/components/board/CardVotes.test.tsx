import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import {
    makeApiError,
    makeBoardSettings,
    makeBoardSnapshot,
    makeCard,
    makeColumn,
    makeMe,
    makeMutationState,
} from "../../test-utils/fixtures";
import { renderWithProviders } from "../../test-utils/render";
import type { BoardSnapshot, CardView } from "../../types/api";
import { CardVotes } from "./CardVotes";

const { useCastVote, useRemoveVote } = vi.hoisted(() => ({ useCastVote: vi.fn(), useRemoveVote: vi.fn() }));

vi.mock("../../hooks/mutations/boards", () => ({ useCastVote, useRemoveVote }));

const top = makeCard({ id: "k1", votes: { total: 2 } });
const under = makeCard({ id: "k2", parent_card_id: "k1", votes: { total: 1 } });

function renderVotes(snapshot: BoardSnapshot, stack: CardView[] = [top, under], canVote = true) {
    return renderWithProviders(<CardVotes snapshot={snapshot} stack={stack} canVote={canVote} />);
}

let cast = makeMutationState();
let remove = makeMutationState();

beforeEach(() => {
    vi.clearAllMocks();
    cast = makeMutationState();
    remove = makeMutationState();
    useCastVote.mockReturnValue(cast);
    useRemoveVote.mockReturnValue(remove);
});

describe("CardVotes", () => {
    it("shows the stack's total and the viewer's own votes on it", () => {
        // when
        renderVotes(makeBoardSnapshot({ me: makeMe({ votes_remaining: 4, my_votes: { k2: 1 } }) }));

        // then
        expect(screen.getByRole("group", { name: "Votes" })).toHaveTextContent("3 votes · 1 yours");
    });

    it("says totals are hidden while the facilitator hides them, still showing the viewer's own", () => {
        // given
        const hidden = [makeCard({ id: "k1", votes: { total: null } })];

        // when
        renderVotes(makeBoardSnapshot({ me: makeMe({ my_votes: { k1: 2 } }) }), hidden);

        // then
        expect(screen.getByRole("group", { name: "Votes" })).toHaveTextContent("Totals hidden · 2 yours");
    });

    it("votes for the stack's top card", async () => {
        // given
        const user = userEvent.setup();
        renderVotes(makeBoardSnapshot());

        // when
        await user.click(screen.getByRole("button", { name: "Vote for this card" }));

        // then
        expect(cast.mutate).toHaveBeenCalledExactlyOnceWith({ cardId: "k1" });
    });

    it("takes a vote back from the card in the stack that holds it", async () => {
        // given
        const user = userEvent.setup();
        renderVotes(makeBoardSnapshot({ me: makeMe({ votes_remaining: 5, my_votes: { k2: 1 } }) }));

        // when
        await user.click(screen.getByRole("button", { name: "Take back a vote" }));

        // then
        expect(remove.mutate).toHaveBeenCalledExactlyOnceWith({ cardId: "k2" });
    });

    it("stops voting once the viewer has no votes left or the card is at its limit", () => {
        // when
        const { unmount } = renderVotes(makeBoardSnapshot({ me: makeMe({ votes_remaining: 0, my_votes: { k1: 6 } }) }));
        const spent = screen.getByRole<HTMLButtonElement>("button", { name: "Vote for this card" }).disabled;
        unmount();
        renderVotes(
            makeBoardSnapshot({
                settings: makeBoardSettings({ max_votes_per_card: 1 }),
                me: makeMe({ votes_remaining: 5, my_votes: { k1: 1 } }),
            }),
        );

        // then
        expect(spent).toBe(true);
        expect(screen.getByRole("button", { name: "Vote for this card" })).toBeDisabled();
    });

    it("stops voting and says why once the viewer has used every vote the card's column allows", () => {
        // given a column that allows one vote, already on this card
        const snapshot = makeBoardSnapshot({
            columns: [makeColumn({ id: top.column_id, max_votes: 1 })],
            cards: [top],
            me: makeMe({ votes_remaining: 5, my_votes: { k1: 1 } }),
        });

        // when
        renderVotes(snapshot, [top]);

        // then
        expect(screen.queryByRole("button", { name: "Vote for this card" })).not.toBeInTheDocument();
        expect(screen.getByRole("button", { name: "No votes left in this column" })).toBeDisabled();
        expect(screen.getByRole("button", { name: "Take back a vote" })).toBeEnabled();
    });

    it("keeps voting open while the column still has votes to give", () => {
        // given
        const snapshot = makeBoardSnapshot({
            columns: [makeColumn({ id: top.column_id, max_votes: 2 })],
            cards: [top],
            me: makeMe({ votes_remaining: 5, my_votes: { k1: 1 } }),
        });

        // when
        renderVotes(snapshot, [top]);

        // then
        expect(screen.getByRole("button", { name: "Vote for this card" })).toBeEnabled();
    });

    it("has nothing to take back when the viewer holds no vote on the stack", () => {
        // when
        renderVotes(makeBoardSnapshot());

        // then
        expect(screen.getByRole("button", { name: "Take back a vote" })).toBeDisabled();
    });

    it("shows the count but no buttons to a viewer who may not vote", () => {
        // when
        renderVotes(makeBoardSnapshot(), [top], false);

        // then
        expect(screen.getByRole("group", { name: "Votes" })).toHaveTextContent("2 votes");
        expect(screen.queryByRole("button")).not.toBeInTheDocument();
    });

    it("shows a refused vote", () => {
        // given
        useCastVote.mockReturnValue(
            makeMutationState({
                error: makeApiError({ status: 409, message: "you have used every vote", code: "vote_limit_reached" }),
            }),
        );

        // when
        renderVotes(makeBoardSnapshot());

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("you have used every vote");
    });
});
