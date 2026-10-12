import { act, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import {
    makeActionItem,
    makeApiError,
    makeBoardSettings,
    makeBoardSnapshot,
    makeBoardView,
    makeCard,
    makeMe,
    makeMutationState,
    makeParticipantViewer,
    makePerson,
    makeSpotlight,
} from "../../test-utils/fixtures";
import { createCardFocus } from "../../hooks/board/useCardFocus";
import { type CardDeleteUndo, CardDeleteUndoContext } from "../../hooks/board/useDeferredCardDelete";
import { renderWithProviders } from "../../test-utils/render";
import type { BoardSnapshot, CardView } from "../../types/api";
import { formatDate } from "../../utils/time";
import { BoardCard } from "./BoardCard";

const mutations = vi.hoisted(() => ({
    useEditCard: vi.fn(),
    useRecolourCard: vi.fn(),
    useDeleteCard: vi.fn(),
    useMoveCard: vi.fn(),
    useCastVote: vi.fn(),
    useRemoveVote: vi.fn(),
    useMergeCard: vi.fn(),
    useUnmergeCard: vi.fn(),
    useCreateComment: vi.fn(),
    useEditComment: vi.fn(),
    useDeleteComment: vi.fn(),
}));

const { useEditCard, useRecolourCard, useDeleteCard, useCastVote, useMergeCard, useUnmergeCard } = mutations;

const actionItemHooks = vi.hoisted(() => ({
    useAddActionItem: vi.fn(),
    useBoardMembership: vi.fn(),
}));

const spotlightHooks = vi.hoisted(() => ({
    useHighlightCard: vi.fn(),
    usePresentCard: vi.fn(),
}));

const polishHooks = vi.hoisted(() => ({
    useAddReaction: vi.fn(),
    useRemoveReaction: vi.fn(),
    usePinCard: vi.fn(),
}));

const clipboard = vi.hoisted(() => ({ copyText: vi.fn() }));

vi.mock("../../hooks/mutations/boards", () => mutations);
vi.mock("../../hooks/mutations/facilitation", () => spotlightHooks);
vi.mock("../../hooks/mutations/actionItems", () => ({ useAddActionItem: actionItemHooks.useAddActionItem }));
vi.mock("../../hooks/mutations/cardPolish", () => polishHooks);
vi.mock("../../hooks/board/useBoardMembership", () => ({ useBoardMembership: actionItemHooks.useBoardMembership }));
vi.mock("../../platform/clipboard", () => clipboard);
vi.mock("./images/CardImage", () => ({
    CardImage: ({ path, alt }: { path: string; alt: string }) => <img src={path} alt={alt} />,
}));
vi.mock("./images/CardImageDialog", () => ({
    CardImageDialog: ({ card, onClose }: { card: CardView; onClose: () => void }) => (
        <div role="dialog" aria-label="Card image">
            <p>{`image for ${card.id}`}</p>
            <button type="button" onClick={onClose}>
                Close
            </button>
        </div>
    ),
}));

const voting = makeBoardView({ stage: "vote", cards_hidden: false });

const discussing = makeBoardView({ stage: "discuss", cards_hidden: false });

const mine = makeCard({ id: "k1", body: "Pair on reviews", colour: "teal" });
const theirs = makeCard({
    id: "k2",
    body: "Fewer meetings",
    is_mine: undefined,
    hidden: false,
    author: { participant_id: "someone", display_name: "Alex" },
});

function renderCard(
    card: CardView,
    snapshot: BoardSnapshot = makeBoardSnapshot({ cards: [mine, theirs] }),
    writable = true,
) {
    return renderWithProviders(
        <BoardCard snapshot={snapshot} card={card} writable={writable} cardFocus={createCardFocus()} />,
    );
}

async function optionsOf(user: ReturnType<typeof userEvent.setup>): Promise<string[]> {
    await user.click(screen.getByRole("button", { name: "Card options" }));

    return within(screen.getByRole("menu"))
        .getAllByRole("menuitem")
        .map(item => item.textContent ?? "");
}

async function choose(user: ReturnType<typeof userEvent.setup>, item: string) {
    await user.click(screen.getByRole("button", { name: "Card options" }));
    await user.click(screen.getByRole("menuitem", { name: item }));
}

beforeEach(() => {
    vi.clearAllMocks();
    for (const hook of Object.values(mutations)) {
        hook.mockReturnValue(makeMutationState());
    }
    for (const hook of Object.values(spotlightHooks)) {
        hook.mockReturnValue(makeMutationState());
    }
    for (const hook of Object.values(polishHooks)) {
        hook.mockReturnValue(makeMutationState());
    }
    clipboard.copyText.mockResolvedValue(undefined);
    actionItemHooks.useAddActionItem.mockReturnValue(makeMutationState());
    actionItemHooks.useBoardMembership.mockReturnValue({
        people: [makePerson({ user_id: "u1", display_name: "Sam" })],
        member: true,
        failure: null,
        retry: vi.fn(),
    });
});

describe("BoardCard", () => {
    it("shows the viewer's own hidden card in full, with its author and a private marker on one line", () => {
        // when
        renderCard(mine);

        // then
        const card = screen.getByRole("article", { name: "Pair on reviews" });
        expect(within(card).getByText(/Only you can see this/)).toHaveTextContent("Sam · Only you can see this");
    });

    it("leaves the author out when the board sends none, as on anonymous boards", () => {
        // when
        renderCard(makeCard({ author: null, hidden: false }));

        // then
        expect(screen.queryByText("Sam")).not.toBeInTheDocument();
        expect(screen.queryByText(/Only you can see this/)).not.toBeInTheDocument();
    });

    it("edits the text and the colour of the viewer's own card in a dialog", async () => {
        // given
        const edit = makeMutationState();
        const recolour = makeMutationState();
        edit.mutate.mockImplementation((_variables, options) => options.onSuccess());
        useEditCard.mockReturnValue(edit);
        useRecolourCard.mockReturnValue(recolour);
        const user = userEvent.setup();
        renderCard(mine);

        // when
        await choose(user, "Edit card");
        const text = screen.getByLabelText("Card text");
        await user.clear(text);
        await user.type(text, "Pair on every review");
        await user.selectOptions(screen.getByLabelText("Card colour"), "Amber");
        await user.click(screen.getByRole("button", { name: "Save card" }));

        // then
        expect(edit.mutate).toHaveBeenCalledExactlyOnceWith(
            { cardId: "k1", body: "Pair on every review", mentions: [] },
            expect.anything(),
        );
        expect(recolour.mutate).toHaveBeenCalledExactlyOnceWith({ cardId: "k1", colour: "amber" }, expect.anything());
    });

    it("marks out each person the card mentions in its text", () => {
        // given
        const sam = makePerson({ user_id: "u1", display_name: "Sam" });

        // when
        renderCard(makeCard({ id: "k5", body: "Thanks @Sam for the demo", mentions: [sam] }));

        // then
        const card = screen.getByRole("article", { name: "Thanks @Sam for the demo" });
        expect(within(card).getByText("@Sam")).toBeInTheDocument();
    });

    it("mentions a member picked with @ while editing the card", async () => {
        // given
        const edit = makeMutationState();
        useEditCard.mockReturnValue(edit);
        const user = userEvent.setup();
        renderCard(mine);

        // when
        await choose(user, "Edit card");
        const text = screen.getByRole("combobox", { name: "Card text" });
        await user.clear(text);
        await user.type(text, "Pair with @Sa");
        await user.click(screen.getByRole("option", { name: "Sam" }));
        await user.click(screen.getByRole("button", { name: "Save card" }));

        // then
        expect(edit.mutate).toHaveBeenCalledExactlyOnceWith(
            { cardId: "k1", body: "Pair with @Sam ", mentions: ["u1"] },
            expect.anything(),
        );
    });

    it("keeps everyone the card still names and drops whoever was taken out of the text", async () => {
        // given
        const edit = makeMutationState();
        useEditCard.mockReturnValue(edit);
        const sam = makePerson({ user_id: "u1", display_name: "Sam" });
        const ali = makePerson({ user_id: "u3", display_name: "Ali" });
        const card = makeCard({ id: "k6", body: "Ask @Sam and @Ali", mentions: [ali, sam] });
        const user = userEvent.setup();
        renderCard(card, makeBoardSnapshot({ cards: [card] }));

        // when
        await choose(user, "Edit card");
        const text = screen.getByRole("combobox", { name: "Card text" });
        await user.clear(text);
        await user.type(text, "Ask @Ali today");
        await user.click(screen.getByRole("button", { name: "Save card" }));

        // then
        expect(edit.mutate).toHaveBeenCalledExactlyOnceWith(
            { cardId: "k6", body: "Ask @Ali today", mentions: ["u3"] },
            expect.anything(),
        );
    });

    it("offers no @ picker to a guest, whose card can name nobody", async () => {
        // given
        actionItemHooks.useBoardMembership.mockReturnValue({
            people: null,
            member: false,
            failure: null,
            retry: vi.fn(),
        });
        const user = userEvent.setup();
        renderCard(mine);

        // when
        await choose(user, "Edit card");

        // then
        expect(screen.getByRole("textbox", { name: "Card text" })).toBeInTheDocument();
        expect(screen.queryByRole("combobox", { name: "Card text" })).not.toBeInTheDocument();
    });

    it("sends only the colour when only the colour changed", async () => {
        // given
        const edit = makeMutationState();
        const recolour = makeMutationState();
        useEditCard.mockReturnValue(edit);
        useRecolourCard.mockReturnValue(recolour);
        const user = userEvent.setup();
        renderCard(mine);

        // when
        await choose(user, "Edit card");
        await user.selectOptions(screen.getByLabelText("Card colour"), "No colour");
        await user.click(screen.getByRole("button", { name: "Save card" }));

        // then
        expect(edit.mutate).not.toHaveBeenCalled();
        expect(recolour.mutate).toHaveBeenCalledExactlyOnceWith({ cardId: "k1", colour: null }, expect.anything());
    });

    it("shows a refused edit in the form", async () => {
        // given
        useEditCard.mockReturnValue(
            makeMutationState({
                isError: true,
                error: makeApiError({ status: 409, message: "cards are locked", code: "cards_locked" }),
            }),
        );
        const user = userEvent.setup();
        renderCard(mine);

        // when
        await choose(user, "Edit card");

        // then
        expect(screen.getByRole("form", { name: "Edit card" })).toHaveTextContent("cards are locked");
    });

    it("deletes the viewer's own card once they confirm", async () => {
        // given
        const removal = makeMutationState();
        useDeleteCard.mockReturnValue(removal);
        const user = userEvent.setup();
        renderCard(mine);

        // when
        await choose(user, "Delete card");
        const dialog = screen.getByRole("dialog", { name: "Delete this card?" });
        await user.click(within(dialog).getByRole("button", { name: "Delete card" }));

        // then
        expect(removal.mutate).toHaveBeenCalledExactlyOnceWith({ cardId: "k1" }, expect.anything());
    });

    it("shows a refused delete in the dialog", async () => {
        // given
        useDeleteCard.mockReturnValue(
            makeMutationState({
                isError: true,
                error: makeApiError({ status: 404, message: "card not found" }),
            }),
        );
        const user = userEvent.setup();
        renderCard(mine);

        // when
        await choose(user, "Delete card");

        // then
        expect(screen.getByRole("dialog", { name: "Delete this card?" })).toHaveTextContent("card not found");
    });

    it("lets a participant move their own card in Write, from a dialog", async () => {
        // given
        const user = userEvent.setup();
        renderCard(mine, makeBoardSnapshot({ viewer: makeParticipantViewer(), cards: [mine, theirs] }));

        // when
        const options = await optionsOf(user);
        await user.click(screen.getByRole("menuitem", { name: "Move card" }));

        // then
        expect(options).toEqual(["Edit card", "Move card", "Add image", "Copy text", "Delete card"]);
        expect(screen.getByRole("form", { name: "Move card" })).toBeInTheDocument();
    });

    it("offers a participant nothing but copying on someone else's card, not even a move", async () => {
        // given
        const user = userEvent.setup();
        renderCard(theirs, makeBoardSnapshot({ viewer: makeParticipantViewer(), cards: [mine, theirs] }));

        // when
        const options = await optionsOf(user);

        // then
        expect(options).toEqual(["Copy text"]);
    });

    it.each(["vote", "discuss", "actions"] as const)(
        "leaves the author no way to edit, move or delete their own card in %s",
        async stage => {
            // given
            const user = userEvent.setup();
            renderCard(
                mine,
                makeBoardSnapshot({
                    board: makeBoardView({ stage, cards_hidden: false }),
                    viewer: makeParticipantViewer(),
                    cards: [mine, theirs],
                    settings: makeBoardSettings({ merging_enabled: false }),
                }),
            );

            // when
            const options = await optionsOf(user);

            // then
            expect(options).toEqual(["Copy text"]);
        },
    );

    it("lets a facilitator delete someone else's card but not edit it", async () => {
        // given
        const user = userEvent.setup();
        renderCard(theirs);

        // when
        const options = await optionsOf(user);

        // then
        expect(options).not.toContain("Edit card");
        expect(options).toContain("Delete card");
    });

    it("offers only copying before the viewer has joined or on an archived board", async () => {
        // given
        const user = userEvent.setup();
        const { unmount } = renderCard(mine, makeBoardSnapshot(), false);
        const beforeJoining = await optionsOf(user);
        unmount();

        // when
        renderCard(mine, makeBoardSnapshot({ board: makeBoardView({ archived_at: "2026-10-02T09:00:00Z" }) }));
        const archived = await optionsOf(user);

        // then
        expect(beforeJoining).toEqual(["Copy text"]);
        expect(archived).toEqual(["Copy text"]);
    });

    it("takes focus when it arrives in a column it was moved to", () => {
        // given
        const cardFocus = createCardFocus();
        cardFocus.request(mine.id);

        // when
        renderWithProviders(<BoardCard snapshot={makeBoardSnapshot()} card={mine} writable cardFocus={cardFocus} />);

        // then
        expect(screen.getByRole("article", { name: "Pair on reviews" })).toHaveFocus();
        expect(cardFocus.claim(mine.id)).toBe(false);
    });

    it("votes for the card and shows the votes on it", async () => {
        // given
        const cast = makeMutationState();
        useCastVote.mockReturnValue(cast);
        const user = userEvent.setup();
        const voted = makeCard({ id: "k1", body: "Pair on reviews", hidden: false, votes: { total: 3 } });
        renderCard(
            voted,
            makeBoardSnapshot({
                board: voting,
                cards: [voted],
                me: makeMe({ votes_remaining: 5, my_votes: { k1: 1 } }),
            }),
        );

        // when
        await user.click(screen.getByRole("button", { name: "Vote for this card" }));

        // then
        expect(cast.mutate).toHaveBeenCalledExactlyOnceWith({ cardId: "k1" });
        expect(screen.getByRole("group", { name: "Votes" })).toHaveTextContent("3 votes · 1 yours");
    });

    it("keeps the totals but drops the vote buttons in Discuss", () => {
        // given
        const voted = makeCard({ id: "k1", body: "Pair on reviews", hidden: false, votes: { total: 3 } });

        // when
        renderCard(
            voted,
            makeBoardSnapshot({
                board: discussing,
                cards: [voted],
                me: makeMe({ votes_remaining: 5, my_votes: { k1: 1 } }),
            }),
        );

        // then
        expect(screen.getByRole("group", { name: "Votes" })).toHaveTextContent("3 votes · 1 yours");
        expect(screen.queryByRole("button", { name: "Vote for this card" })).not.toBeInTheDocument();
        expect(screen.queryByRole("button", { name: "Take back a vote" })).not.toBeInTheDocument();
    });

    it("shows no votes at all in Write, even while voting is open", () => {
        // when
        renderCard(mine, makeBoardSnapshot({ cards: [mine], settings: makeBoardSettings({ voting_open: true }) }));

        // then
        expect(screen.queryByRole("group", { name: "Votes" })).not.toBeInTheDocument();
    });

    it("offers merging only in Discuss, once cards are shown and merging is on", async () => {
        // given
        const user = userEvent.setup();
        const shown = discussing;

        // when
        const { unmount } = renderCard(mine, makeBoardSnapshot({ board: shown, cards: [mine, theirs] }));
        const offered = await optionsOf(user);
        unmount();
        renderCard(
            mine,
            makeBoardSnapshot({
                board: shown,
                cards: [mine, theirs],
                settings: makeBoardSettings({ merging_enabled: false }),
            }),
        );
        const switchedOff = await optionsOf(user);

        // then
        expect(offered).toContain("Merge into another card");
        expect(switchedOff).not.toContain("Merge into another card");
    });

    it("merges into the card picked in the merge dialog", async () => {
        // given
        const merge = makeMutationState();
        useMergeCard.mockReturnValue(merge);
        const user = userEvent.setup();
        renderCard(mine, makeBoardSnapshot({ board: discussing, cards: [mine, theirs] }));

        // when
        await choose(user, "Merge into another card");
        await user.click(screen.getByRole("button", { name: "Merge cards" }));

        // then
        expect(merge.mutate).toHaveBeenCalledExactlyOnceWith({ cardId: "k1", targetCardId: "k2" }, expect.anything());
    });

    it("shows the cards merged into it and lets them be taken back out", async () => {
        // given
        const unmerge = makeMutationState();
        useUnmergeCard.mockReturnValue(unmerge);
        const user = userEvent.setup();
        const under = { ...theirs, parent_card_id: "k1" };
        const snapshot = makeBoardSnapshot({ board: discussing, cards: [mine, under] });
        renderWithProviders(
            <BoardCard snapshot={snapshot} card={mine} merged={[under]} writable cardFocus={createCardFocus()} />,
        );

        // when
        const stacked = within(screen.getByRole("list", { name: "Merged cards" }));
        await user.click(stacked.getByRole("button", { name: 'Unmerge "Fewer meetings"' }));

        // then
        expect(stacked.getByText("Fewer meetings")).toBeInTheDocument();
        expect(stacked.getByText("Alex")).toBeInTheDocument();
        expect(unmerge.mutate).toHaveBeenCalledExactlyOnceWith({ cardId: "k2" });
    });

    it("leaves out comments while they are switched off, and votes while voting is closed and none were cast", () => {
        // when
        renderCard(
            mine,
            makeBoardSnapshot({
                cards: [mine],
                settings: makeBoardSettings({ comments_enabled: false, voting_open: false }),
            }),
        );

        // then
        expect(screen.queryByRole("button", { name: "Comment" })).not.toBeInTheDocument();
        expect(screen.queryByRole("group", { name: "Votes" })).not.toBeInTheDocument();
    });

    it("never offers a vote before the viewer has joined", () => {
        // when
        renderCard(mine, makeBoardSnapshot({ board: voting }), false);

        // then
        expect(screen.queryByRole("button", { name: "Vote for this card" })).not.toBeInTheDocument();
        expect(screen.getByRole("group", { name: "Votes" })).toHaveTextContent("0 votes");
    });

    it("makes a revealed card into an action item in the Actions stage, its text filled in", async () => {
        // given
        const add = makeMutationState();
        actionItemHooks.useAddActionItem.mockReturnValue(add);
        const user = userEvent.setup();
        const card = { ...theirs, body: "Fewer\nmeetings" };
        renderCard(
            card,
            makeBoardSnapshot({
                board: makeBoardView({ stage: "actions", cards_hidden: false }),
                viewer: makeParticipantViewer(),
                cards: [card],
            }),
        );

        // when
        await user.click(screen.getByRole("button", { name: "+ Action" }));
        const dialog = within(screen.getByRole("dialog", { name: "Make action item" }));
        await user.click(dialog.getByRole("button", { name: "Add action item" }));

        // then
        expect(dialog.getByRole("combobox", { name: "Action item" })).toHaveValue("Fewer meetings");
        expect(add.mutate).toHaveBeenCalledWith(
            {
                title: "Fewer meetings",
                assignee_user_id: null,
                due_on: null,
                mentions: [],
                source_card_id: "k2",
            },
            expect.anything(),
        );
    });

    it.each(["write", "vote", "discuss"] as const)("offers no way to make a card an action item in %s", async stage => {
        // given
        const user = userEvent.setup();

        // when
        renderCard(
            theirs,
            makeBoardSnapshot({
                board: makeBoardView({ stage, cards_hidden: false }),
                viewer: makeParticipantViewer(),
                cards: [theirs],
                settings: makeBoardSettings({ merging_enabled: false }),
            }),
        );

        // then
        expect(screen.queryByRole("button", { name: "+ Action" })).not.toBeInTheDocument();
        await user.click(screen.getByRole("button", { name: "Card options" }));
        expect(
            within(screen.getByRole("menu"))
                .getAllByRole("menuitem")
                .map(item => item.textContent),
        ).toEqual(["Copy text"]);
    });

    it("keeps Make action item out of the card's menu", async () => {
        // given
        const user = userEvent.setup();
        renderCard(
            theirs,
            makeBoardSnapshot({ board: makeBoardView({ stage: "actions", cards_hidden: false }), cards: [theirs] }),
        );

        // when
        const options = await optionsOf(user);

        // then
        expect(options).not.toContain("Make action item");
        expect(screen.getByRole("button", { name: "+ Action" })).toBeInTheDocument();
    });

    it("offers no + Action before the viewer has joined, or on a card only its author can see", () => {
        // given
        const hiddenCard = makeCard({ id: "k3", body: "Secret", hidden: true });
        const snapshot = makeBoardSnapshot({
            board: makeBoardView({ stage: "actions", cards_hidden: false }),
            viewer: makeParticipantViewer(),
            cards: [theirs, hiddenCard],
        });

        // when
        renderCard(theirs, snapshot, false);
        renderCard(hiddenCard, snapshot);

        // then
        expect(screen.queryByRole("button", { name: "+ Action" })).not.toBeInTheDocument();
    });

    it("counts the action items made from a card, and still offers another", () => {
        // when
        renderCard(
            theirs,
            makeBoardSnapshot({
                board: makeBoardView({ stage: "actions", cards_hidden: false }),
                viewer: makeParticipantViewer(),
                cards: [theirs],
                action_items: [
                    makeActionItem({ id: "i1", source_card_id: "k2" }),
                    makeActionItem({ id: "i2", source_card_id: "k2" }),
                    makeActionItem({ id: "i3", source_card_id: "k1" }),
                ],
            }),
        );

        // then
        expect(screen.getByText("2 actions")).toBeInTheDocument();
        expect(screen.getByRole("button", { name: "+ Action" })).toBeInTheDocument();
    });

    it("shows one action on a card after the Actions stage, with no way to add more", () => {
        // when
        renderCard(
            theirs,
            makeBoardSnapshot({
                board: makeBoardView({ stage: "discuss", cards_hidden: false }),
                viewer: makeParticipantViewer(),
                cards: [theirs],
                settings: makeBoardSettings({ comments_enabled: false }),
                action_items: [makeActionItem({ source_card_id: "k2" })],
            }),
        );

        // then
        expect(screen.getByText("1 action")).toBeInTheDocument();
        expect(screen.queryByRole("button", { name: "+ Action" })).not.toBeInTheDocument();
    });

    it("scrolls to the card, focuses it and highlights it when it is revealed", () => {
        // given
        const cardFocus = createCardFocus();
        renderWithProviders(<BoardCard snapshot={makeBoardSnapshot()} card={mine} writable cardFocus={cardFocus} />);
        const article = screen.getByRole("article", { name: "Pair on reviews" });
        const scrolled = vi.fn();
        article.scrollIntoView = scrolled;

        // when
        act(() => {
            cardFocus.reveal("k1");
        });

        // then
        expect(scrolled).toHaveBeenCalledWith({ block: "center" });
        expect(article).toHaveFocus();
    });

    it("stops answering reveals once the card leaves the board", () => {
        // given
        const cardFocus = createCardFocus();
        const { unmount } = renderWithProviders(
            <BoardCard snapshot={makeBoardSnapshot()} card={mine} writable cardFocus={cardFocus} />,
        );

        // when
        unmount();

        // then
        expect(cardFocus.reveal("k1")).toBe(false);
    });

    it("leaves focus alone when another card was moved", () => {
        // given
        const cardFocus = createCardFocus();
        cardFocus.request("someone-else");

        // when
        renderWithProviders(<BoardCard snapshot={makeBoardSnapshot()} card={mine} writable cardFocus={cardFocus} />);

        // then
        expect(screen.getByRole("article", { name: "Pair on reviews" })).not.toHaveFocus();
    });

    it("tags the highlighted card for everyone, a participant included", () => {
        // given
        const snapshot = makeBoardSnapshot({
            board: discussing,
            viewer: makeParticipantViewer(),
            cards: [mine, theirs],
            spotlight: makeSpotlight({ highlighted_card_id: "k2" }),
        });

        // when
        renderCard(theirs, snapshot);

        // then
        const card = screen.getByRole("article", { name: "Fewer meetings" });
        expect(within(card).getByText("Highlighted")).toBeInTheDocument();
    });

    it("offers a participant no way to highlight or present", async () => {
        // given
        const user = userEvent.setup();
        const snapshot = makeBoardSnapshot({
            board: discussing,
            viewer: makeParticipantViewer(),
            cards: [mine, theirs],
        });
        renderCard(theirs, snapshot);

        // when
        const options = await optionsOf(user);

        // then
        expect(options).toEqual(["Merge into another card", "Copy text"]);
    });

    it("tags a stack whose merged card is the one highlighted", () => {
        // given
        const under = makeCard({ id: "k3", body: "Shorter standups", hidden: false, parent_card_id: "k2" });
        const snapshot = makeBoardSnapshot({
            board: discussing,
            cards: [theirs, under],
            spotlight: makeSpotlight({ highlighted_card_id: "k3" }),
        });

        // when
        renderWithProviders(
            <BoardCard snapshot={snapshot} card={theirs} merged={[under]} writable cardFocus={createCardFocus()} />,
        );

        // then
        expect(screen.getByText("Highlighted")).toBeInTheDocument();
    });

    it("leaves a card that is not highlighted untagged", () => {
        // when
        renderCard(theirs, makeBoardSnapshot({ board: discussing, cards: [mine, theirs] }));

        // then
        expect(screen.queryByText("Highlighted")).not.toBeInTheDocument();
    });

    it("lets a facilitator highlight a shown card for everyone and present from it", async () => {
        // given
        const user = userEvent.setup();
        const highlight = makeMutationState();
        const present = makeMutationState();
        spotlightHooks.useHighlightCard.mockReturnValue(highlight);
        spotlightHooks.usePresentCard.mockReturnValue(present);
        renderCard(theirs, makeBoardSnapshot({ board: discussing, cards: [mine, theirs] }));

        // when
        await choose(user, "Highlight for everyone");
        await choose(user, "Present from this card");

        // then
        expect(highlight.mutate).toHaveBeenCalledExactlyOnceWith({ cardId: "k2" });
        expect(present.mutate).toHaveBeenCalledExactlyOnceWith({ cardId: "k2" });
    });

    it("lets a facilitator remove the highlight from the highlighted card", async () => {
        // given
        const user = userEvent.setup();
        const highlight = makeMutationState();
        spotlightHooks.useHighlightCard.mockReturnValue(highlight);
        const snapshot = makeBoardSnapshot({
            board: discussing,
            cards: [mine, theirs],
            spotlight: makeSpotlight({ highlighted_card_id: "k2" }),
        });
        renderCard(theirs, snapshot);

        // when
        const options = await optionsOf(user);
        await user.click(screen.getByRole("menuitem", { name: "Remove highlight" }));

        // then
        expect(options).not.toContain("Highlight for everyone");
        expect(highlight.mutate).toHaveBeenCalledExactlyOnceWith({ cardId: null });
    });

    it("offers no spotlight on a card only its author can see, or while its column's cards are hidden", async () => {
        // given
        const user = userEvent.setup();
        const shownCard = makeCard({ ...theirs, id: "k4" });
        const { unmount } = renderCard(mine, makeBoardSnapshot({ board: discussing, cards: [mine, theirs] }));
        const ownHidden = await optionsOf(user);
        unmount();

        // when
        renderCard(shownCard, makeBoardSnapshot({ cards: [mine, shownCard] }));
        const columnHidden = await optionsOf(user);

        // then
        expect(ownHidden).not.toContain("Present from this card");
        expect(columnHidden).not.toContain("Present from this card");
        expect(columnHidden).not.toContain("Highlight for everyone");
    });

    it("shows a refused highlight as a notice", () => {
        // given
        spotlightHooks.useHighlightCard.mockReturnValue(
            makeMutationState({
                error: makeApiError({ status: 409, message: "card not shown", code: "card_not_shown" }),
            }),
        );

        // when
        renderCard(theirs, makeBoardSnapshot({ board: discussing, cards: [mine, theirs] }));

        // then
        expect(screen.getByText("Only a card everyone can see can be presented or highlighted.")).toBeInTheDocument();
    });

    it("tags a pinned card and lets a facilitator unpin it", async () => {
        // given
        const user = userEvent.setup();
        const pin = makeMutationState();
        polishHooks.usePinCard.mockReturnValue(pin);
        const pinned = makeCard({ ...theirs, pinned: true });
        renderCard(pinned, makeBoardSnapshot({ board: discussing, cards: [mine, pinned] }));

        // when
        const options = await optionsOf(user);
        await user.click(screen.getByRole("menuitem", { name: "Unpin" }));

        // then
        expect(within(screen.getByRole("article", { name: "Fewer meetings" })).getByText("Pinned")).toBeInTheDocument();
        expect(options).not.toContain("Pin to top");
        expect(pin.mutate).toHaveBeenCalledExactlyOnceWith({ cardId: "k2", pinned: false });
    });

    it("lets a facilitator pin a card everyone can see to the top", async () => {
        // given
        const user = userEvent.setup();
        const pin = makeMutationState();
        polishHooks.usePinCard.mockReturnValue(pin);
        renderCard(theirs, makeBoardSnapshot({ board: discussing, cards: [mine, theirs] }));

        // when
        await choose(user, "Pin to top");

        // then
        expect(screen.queryByText("Pinned")).not.toBeInTheDocument();
        expect(pin.mutate).toHaveBeenCalledExactlyOnceWith({ cardId: "k2", pinned: true });
    });

    it("offers no pin to a participant, or on a card only its author can see", async () => {
        // given
        const user = userEvent.setup();
        const { unmount } = renderCard(
            theirs,
            makeBoardSnapshot({ board: discussing, viewer: makeParticipantViewer(), cards: [mine, theirs] }),
        );
        const participant = await optionsOf(user);
        unmount();

        // when
        renderCard(mine);
        const ownHidden = await optionsOf(user);

        // then
        expect(participant).not.toContain("Pin to top");
        expect(ownHidden).not.toContain("Pin to top");
    });

    it("says why a pin was refused", () => {
        // given
        polishHooks.usePinCard.mockReturnValue(
            makeMutationState({
                error: makeApiError({
                    status: 409,
                    message: "only a card everyone can see can be reacted to or pinned",
                    code: "card_not_public",
                }),
            }),
        );

        // when
        renderCard(theirs, makeBoardSnapshot({ board: discussing, cards: [mine, theirs] }));

        // then
        expect(screen.getByText("only a card everyone can see can be reacted to or pinned")).toBeInTheDocument();
    });

    it("shows the date the card was written once the board shows card dates", () => {
        // given
        const snapshot = makeBoardSnapshot({ settings: makeBoardSettings({ show_card_dates: true }) });

        // when
        renderCard(mine, snapshot);

        // then
        const card = screen.getByRole("article", { name: "Pair on reviews" });
        expect(within(card).getByText(/Only you can see this/)).toHaveTextContent(
            `Sam · ${formatDate("2026-10-01T09:05:00Z")} · Only you can see this`,
        );
    });

    it("leaves the date out while card dates are off, and when the board sends none", () => {
        // given
        const dated = makeBoardSnapshot({ settings: makeBoardSettings({ show_card_dates: true }) });
        const written = formatDate("2026-10-01T09:05:00Z");

        // when
        const { unmount } = renderCard(mine);
        const whileOff = screen.queryByText(written);
        unmount();
        renderCard(makeCard({ ...theirs, author: null, created_at: undefined }), dated);

        // then
        expect(whileOff).not.toBeInTheDocument();
        expect(screen.queryByText(written)).not.toBeInTheDocument();
    });

    it("copies the card's text and says so", async () => {
        // given
        const user = userEvent.setup();
        renderCard(theirs, makeBoardSnapshot({ board: discussing, cards: [mine, theirs] }));

        // when
        await choose(user, "Copy text");

        // then
        expect(clipboard.copyText).toHaveBeenCalledExactlyOnceWith("- Fewer meetings");
        expect(await screen.findByText("Card text copied.")).toBeInTheDocument();
    });

    it("says why copying the card failed", async () => {
        // given
        clipboard.copyText.mockRejectedValue(new Error("This browser does not let the page copy text."));
        const user = userEvent.setup();
        renderCard(theirs, makeBoardSnapshot({ board: discussing, cards: [mine, theirs] }));

        // when
        await choose(user, "Copy text");

        // then
        expect(await screen.findByText("This browser does not let the page copy text.")).toBeInTheDocument();
    });

    it("shows the image on the card", () => {
        // when
        renderCard(makeCard({ ...theirs, image_path: "/uploads/b1/m1.webp" }));

        // then
        expect(screen.getByRole("img", { name: "Image on the card" })).toHaveAttribute("src", "/uploads/b1/m1.webp");
    });

    it("lets the author add an image to their card from a dialog", async () => {
        // given
        const user = userEvent.setup();
        renderCard(mine);

        // when
        await choose(user, "Add image");
        const dialog = screen.getByRole("dialog", { name: "Card image" });
        await user.click(within(dialog).getByRole("button", { name: "Close" }));

        // then
        expect(dialog).toHaveTextContent("image for k1");
        expect(screen.queryByRole("dialog", { name: "Card image" })).not.toBeInTheDocument();
    });

    it("offers to change an image already on the card, and no image to someone who cannot edit it", async () => {
        // given
        const user = userEvent.setup();
        const { unmount } = renderCard(makeCard({ ...mine, image_path: "/uploads/b1/m1.webp" }));
        const own = await optionsOf(user);
        unmount();

        // when
        renderCard(theirs, makeBoardSnapshot({ viewer: makeParticipantViewer(), cards: [mine, theirs] }));
        const others = await optionsOf(user);

        // then
        expect(own).toContain("Change image");
        expect(own).not.toContain("Add image");
        expect(others).not.toContain("Add image");
    });

    it("shows the reactions on a card everyone can see", () => {
        // given
        const reacted = makeCard({ ...theirs, reactions: [{ reaction: "party", count: 2 }] });

        // when
        renderCard(reacted, makeBoardSnapshot({ board: discussing, cards: [mine, reacted] }));

        // then
        expect(screen.getByRole("button", { name: "Party: 2 reactions" })).toBeInTheDocument();
        expect(screen.getByRole("button", { name: /Add reaction/ })).toBeInTheDocument();
    });

    describe("with undo for deletes", () => {
        function deferredDeletes(overrides: Partial<CardDeleteUndo> = {}): CardDeleteUndo {
            return {
                hidden: new Set(),
                waiting: [],
                schedule: vi.fn(),
                undo: vi.fn(),
                error: null,
                dismissError: vi.fn(),
                ...overrides,
            };
        }

        function renderWithUndo(card: CardView, deferred: CardDeleteUndo) {
            return renderWithProviders(
                <CardDeleteUndoContext.Provider value={deferred}>
                    <BoardCard
                        snapshot={makeBoardSnapshot({ cards: [mine, theirs] })}
                        card={card}
                        writable
                        cardFocus={createCardFocus()}
                    />
                </CardDeleteUndoContext.Provider>,
            );
        }

        it("hands the delete to the undo window once confirmed, and sends nothing itself", async () => {
            // given
            const user = userEvent.setup();
            const removal = makeMutationState();
            useDeleteCard.mockReturnValue(removal);
            const deferred = deferredDeletes();
            renderWithUndo(mine, deferred);

            // when
            await choose(user, "Delete card");
            const dialog = screen.getByRole("dialog", { name: "Delete this card?" });
            const warning = dialog.textContent;
            await user.click(within(dialog).getByRole("button", { name: "Delete card" }));

            // then
            expect(warning).toContain("You can undo it for a few seconds afterwards.");
            expect(deferred.schedule).toHaveBeenCalledExactlyOnceWith("k1");
            expect(removal.mutate).not.toHaveBeenCalled();
            expect(screen.queryByRole("dialog", { name: "Delete this card?" })).not.toBeInTheDocument();
        });

        it("renders nothing while its delete waits", () => {
            // when
            renderWithUndo(mine, deferredDeletes({ hidden: new Set(["k1"]), waiting: ["k1"] }));

            // then
            expect(screen.queryByRole("article", { name: "Pair on reviews" })).not.toBeInTheDocument();
        });
    });
});
