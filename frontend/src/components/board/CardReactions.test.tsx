import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import {
    makeApiError,
    makeBoardSnapshot,
    makeBoardView,
    makeBoardViewer,
    makeCard,
    makeMutationState,
    type MutationStateFixture,
} from "../../test-utils/fixtures";
import { renderWithProviders } from "../../test-utils/render";
import type { BoardSnapshot, CardView } from "../../types/api";
import { CardReactions } from "./CardReactions";

const polish = vi.hoisted(() => ({ useAddReaction: vi.fn(), useRemoveReaction: vi.fn(), usePinCard: vi.fn() }));

vi.mock("../../hooks/mutations/cardPolish", () => polish);

const shown = makeCard({ id: "k2", hidden: false, is_mine: undefined });

const reacted = makeCard({
    ...shown,
    reactions: [
        { reaction: "thumbs_up", count: 3 },
        { reaction: "heart", count: 1 },
    ],
    my_reactions: ["heart"],
});

let add: MutationStateFixture;
let remove: MutationStateFixture;

function renderReactions(card: CardView, snapshot: BoardSnapshot = makeBoardSnapshot(), writable = true) {
    return renderWithProviders(<CardReactions snapshot={snapshot} card={card} writable={writable} />);
}

beforeEach(() => {
    vi.clearAllMocks();
    add = makeMutationState();
    remove = makeMutationState();
    polish.useAddReaction.mockReturnValue(add);
    polish.useRemoveReaction.mockReturnValue(remove);
});

describe("CardReactions", () => {
    it("shows each reaction's count and marks the viewer's own", () => {
        // when
        renderReactions(reacted);

        // then
        const group = screen.getByRole("group", { name: "Reactions" });
        expect(within(group).getByRole("button", { name: "Thumbs up: 3 reactions" })).toHaveAttribute(
            "aria-pressed",
            "false",
        );
        expect(within(group).getByRole("button", { name: "Love: 1 reaction, yours included" })).toHaveAttribute(
            "aria-pressed",
            "true",
        );
        expect(within(group).getByRole("button", { name: "Thumbs up: 3 reactions" })).toHaveTextContent("👍3");
    });

    it("takes the viewer's own reaction back and adds one they have not given", async () => {
        // given
        const user = userEvent.setup();
        renderReactions(reacted);

        // when
        await user.click(screen.getByRole("button", { name: "Love: 1 reaction, yours included" }));
        await user.click(screen.getByRole("button", { name: "Thumbs up: 3 reactions" }));

        // then
        expect(polish.useAddReaction).toHaveBeenCalledWith(makeBoardSnapshot().board_id);
        expect(remove.mutate).toHaveBeenCalledExactlyOnceWith({ cardId: "k2", reaction: "heart" });
        expect(add.mutate).toHaveBeenCalledExactlyOnceWith({ cardId: "k2", reaction: "thumbs_up" });
    });

    it("offers all fifteen reactions from Add reaction", async () => {
        // given
        const user = userEvent.setup();
        renderReactions(shown);

        // when
        await user.click(screen.getByRole("button", { name: /Add reaction/ }));
        const options = within(screen.getByRole("menu")).getAllByRole("menuitem");
        await user.click(screen.getByRole("menuitem", { name: "🚀 Rocket" }));

        // then
        expect(options).toHaveLength(15);
        expect(options[0]).toHaveTextContent("👍 Thumbs up");
        expect(options[14]).toHaveTextContent("⭐ Star");
        expect(add.mutate).toHaveBeenCalledExactlyOnceWith({ cardId: "k2", reaction: "rocket" });
    });

    it("takes a reaction back when it is picked again from the menu", async () => {
        // given
        const user = userEvent.setup();
        renderReactions(reacted);

        // when
        await user.click(screen.getByRole("button", { name: /Add reaction/ }));
        await user.click(screen.getByRole("menuitem", { name: "❤️ Love" }));

        // then
        expect(remove.mutate).toHaveBeenCalledExactlyOnceWith({ cardId: "k2", reaction: "heart" });
        expect(add.mutate).not.toHaveBeenCalled();
    });

    it("renders nothing on a card nobody reacted to when the viewer cannot react", () => {
        // when
        renderReactions(shown, makeBoardSnapshot(), false);

        // then
        expect(screen.queryByRole("group", { name: "Reactions" })).not.toBeInTheDocument();
    });

    it("renders nothing on the viewer's own card while only they can see it", () => {
        // when
        renderReactions(makeCard({ id: "k1", hidden: true }));

        // then
        expect(screen.queryByRole("group", { name: "Reactions" })).not.toBeInTheDocument();
    });

    it("shows the counts but no way to react to someone without a seat", () => {
        // when
        renderReactions(reacted, makeBoardSnapshot({ viewer: makeBoardViewer({ participant_id: null }) }));

        // then
        expect(screen.getByRole("button", { name: "Thumbs up: 3 reactions" })).toBeDisabled();
        expect(screen.queryByRole("button", { name: /Add reaction/ })).not.toBeInTheDocument();
    });

    it("shows the counts but no way to react on an archived board", () => {
        // when
        renderReactions(
            reacted,
            makeBoardSnapshot({ board: makeBoardView({ cards_hidden: false, archived_at: "2026-10-02T09:00:00Z" }) }),
        );

        // then
        expect(screen.getByRole("button", { name: "Love: 1 reaction, yours included" })).toBeDisabled();
        expect(screen.queryByRole("button", { name: /Add reaction/ })).not.toBeInTheDocument();
    });

    it("says why a reaction was refused", () => {
        // given
        polish.useAddReaction.mockReturnValue(
            makeMutationState({
                error: makeApiError({
                    status: 409,
                    message: "only a card everyone can see can be reacted to or pinned",
                    code: "card_not_public",
                }),
            }),
        );

        // when
        renderReactions(shown);

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("only a card everyone can see can be reacted to or pinned");
    });
});
