import { fireEvent, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { createCardFocus } from "../../../hooks/board/useCardFocus";
import {
    makeActionItem,
    makeBoardSettings,
    makeBoardSnapshot,
    makeBoardView,
    makeCard,
    makeComment,
    makeMutationState,
    makeParticipantViewer,
    makeSpotlight,
} from "../../../test-utils/fixtures";
import { renderWithProviders } from "../../../test-utils/render";
import type { BoardSnapshot, BoardStage } from "../../../types/api";
import { PresentationView } from "./PresentationView";

const hooks = vi.hoisted(() => ({
    usePresentCard: vi.fn(),
    useBoardMembership: vi.fn(),
}));

vi.mock("../../../hooks/mutations/facilitation", () => ({ usePresentCard: hooks.usePresentCard }));
vi.mock("../../../hooks/board/useBoardMembership", () => ({ useBoardMembership: hooks.useBoardMembership }));

const START = "00000000-0000-4000-8000-000000000c01";

const first = makeCard({ id: "first", column_id: START, position: "a0", body: "Ship smaller", hidden: false });
const presented = makeCard({
    id: "presented",
    column_id: START,
    position: "a1",
    body: "Fewer meetings",
    hidden: false,
    is_mine: undefined,
    votes: { total: 3 },
    author: { participant_id: "00000000-0000-4000-8000-0000000000f2", display_name: "Alex" },
});
const last = makeCard({ id: "last", column_id: START, position: "a2", body: "Pair more", hidden: false });

function presenting(stage: BoardStage = "discuss", overrides: Partial<BoardSnapshot> = {}): BoardSnapshot {
    return makeBoardSnapshot({
        board: makeBoardView({ stage, cards_hidden: false }),
        cards: [first, presented, last],
        hidden_counts: {},
        spotlight: makeSpotlight({ presenting_card_id: "presented" }),
        ...overrides,
    });
}

function renderPresentation(snapshot: BoardSnapshot = presenting(), onLeave = vi.fn()) {
    renderWithProviders(
        <PresentationView snapshot={snapshot} writable cardFocus={createCardFocus()} onLeave={onLeave} />,
    );

    return onLeave;
}

let present = makeMutationState();

beforeEach(() => {
    vi.clearAllMocks();
    present = makeMutationState();
    hooks.usePresentCard.mockReturnValue(present);
    hooks.useBoardMembership.mockReturnValue({ people: [], member: true, failure: null, retry: vi.fn() });
});

describe("PresentationView", () => {
    it("shows the presented card large with its place, its author and its votes", () => {
        // when
        renderPresentation();

        // then
        const presentation = screen.getByRole("region", { name: "Presentation" });
        expect(within(presentation).getByText("Card 2 of 3")).toBeInTheDocument();
        const card = within(presentation).getByRole("article", { name: "Presented card" });
        expect(card).toHaveTextContent("Fewer meetings");
        expect(within(card).getByText("Alex")).toBeInTheDocument();
        expect(within(card).getByText("3 votes")).toBeInTheDocument();
        expect(screen.getByRole("status")).toHaveTextContent("Presenting card 2 of 3.");
    });

    it("leaves the vote total out while totals are hidden", () => {
        // given
        const hiddenTotal = { ...presented, votes: { total: null } };

        // when
        renderPresentation(presenting("discuss", { cards: [first, hiddenTotal, last] }));

        // then
        expect(screen.queryByText(/votes?$/)).not.toBeInTheDocument();
    });

    it("lets the facilitator step back, step on and end the presentation", async () => {
        // given
        const user = userEvent.setup();
        renderPresentation();

        // when
        await user.click(screen.getByRole("button", { name: "Previous" }));
        await user.click(screen.getByRole("button", { name: "Next" }));
        await user.click(screen.getByRole("button", { name: "End presentation" }));

        // then
        expect(present.mutate).toHaveBeenNthCalledWith(1, { cardId: "first" });
        expect(present.mutate).toHaveBeenNthCalledWith(2, { cardId: "last" });
        expect(present.mutate).toHaveBeenNthCalledWith(3, { cardId: null });
        expect(screen.queryByRole("button", { name: "Leave presentation" })).not.toBeInTheDocument();
    });

    it("steps with the arrow keys for the facilitator", () => {
        // given
        renderPresentation();

        // when
        fireEvent.keyDown(window, { key: "ArrowRight" });

        // then
        expect(present.mutate).toHaveBeenCalledWith({ cardId: "last" });
    });

    it("stops Next at the last card", () => {
        // when
        renderPresentation(presenting("discuss", { spotlight: makeSpotlight({ presenting_card_id: "last" }) }));

        // then
        expect(screen.getByRole("button", { name: "Next" })).toBeDisabled();
        expect(screen.getByRole("button", { name: "Previous" })).toBeEnabled();
    });

    it("lets a follower leave the presentation, with a button or Escape, but never steer it", async () => {
        // given
        const user = userEvent.setup();
        const onLeave = renderPresentation(presenting("discuss", { viewer: makeParticipantViewer() }));

        // when
        await user.click(screen.getByRole("button", { name: "Leave presentation" }));
        fireEvent.keyDown(window, { key: "Escape" });
        fireEvent.keyDown(window, { key: "ArrowRight" });

        // then
        expect(onLeave).toHaveBeenCalledTimes(2);
        expect(present.mutate).not.toHaveBeenCalled();
        expect(screen.queryByRole("button", { name: "Next" })).not.toBeInTheDocument();
        expect(screen.queryByRole("button", { name: "End presentation" })).not.toBeInTheDocument();
    });

    it("shows the merged cards and the comments of the presented stack", () => {
        // given
        const merged = makeCard({
            id: "under",
            column_id: START,
            parent_card_id: "presented",
            position: "a3",
            body: "No Friday calls",
            hidden: false,
        });
        const snapshot = presenting("discuss", {
            cards: [first, presented, merged, last],
            comments: [makeComment({ id: "c1", card_id: "under", body: "Agreed" })],
        });

        // when
        renderPresentation(snapshot);

        // then
        expect(screen.getByRole("list", { name: "Merged cards" })).toHaveTextContent("No Friday calls");
        expect(screen.getByRole("button", { name: "1 comment" })).toBeInTheDocument();
    });

    it("offers vote buttons only in Vote", () => {
        // when
        renderPresentation(presenting("vote", { viewer: makeParticipantViewer() }));

        // then
        expect(screen.getByRole("group", { name: "Votes" })).toBeInTheDocument();
        expect(screen.getByRole("button", { name: "Vote for this card" })).toBeInTheDocument();
    });

    it("lists the stack's action items and offers + Action in Actions", () => {
        // given
        const snapshot = presenting("actions", {
            action_items: [
                makeActionItem({ id: "mine", title: "Cancel the Monday sync", source_card_id: "presented" }),
                makeActionItem({ id: "other", title: "Book the retro room", source_card_id: "first" }),
            ],
        });

        // when
        renderPresentation(snapshot);

        // then
        expect(screen.getByText("Cancel the Monday sync")).toBeInTheDocument();
        expect(screen.queryByText("Book the retro room")).not.toBeInTheDocument();
        expect(screen.getByRole("button", { name: "+ Action" })).toBeInTheDocument();
    });

    it("offers no + Action outside Actions", () => {
        // when
        renderPresentation(presenting("discuss", { settings: makeBoardSettings({ comments_enabled: false }) }));

        // then
        expect(screen.queryByRole("button", { name: "+ Action" })).not.toBeInTheDocument();
    });

    it("says so when the presented card is no longer on the board", () => {
        // when
        renderPresentation(presenting("discuss", { cards: [first, last] }));

        // then
        expect(screen.getByText("This card is no longer on the board")).toBeInTheDocument();
        expect(screen.getByRole("button", { name: "End presentation" })).toBeInTheDocument();
    });
});
