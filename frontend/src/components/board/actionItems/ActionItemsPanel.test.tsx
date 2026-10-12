import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { BoardMembership } from "../../../hooks/board/useBoardMembership";
import { createCardFocus } from "../../../hooks/board/useCardFocus";
import {
    makeActionItem,
    makeApiError,
    makeBoardSnapshot,
    makeBoardView,
    makeBoardViewer,
    makeCard,
    makeMutationState,
    makeParticipantViewer,
    makePerson,
    type MutationStateFixture,
} from "../../../test-utils/fixtures";
import { renderWithProviders } from "../../../test-utils/render";
import type { BoardSnapshot, BoardStage } from "../../../types/api";
import { ActionItemsPanel } from "./ActionItemsPanel";

const hooks = vi.hoisted(() => ({
    useMarkActionItem: vi.fn(),
    useEditActionItem: vi.fn(),
    useDeleteActionItem: vi.fn(),
    useBoardMembership: vi.fn(),
}));

vi.mock("../../../hooks/mutations/actionItems", () => ({
    useMarkActionItem: hooks.useMarkActionItem,
    useEditActionItem: hooks.useEditActionItem,
    useDeleteActionItem: hooks.useDeleteActionItem,
}));
vi.mock("../../../hooks/board/useBoardMembership", () => ({ useBoardMembership: hooks.useBoardMembership }));

const sam = makePerson({ user_id: "u1", display_name: "Sam" });
const ali = makePerson({ user_id: "u3", display_name: "Ali Khan" });

const booked = makeActionItem({ id: "i1", title: "Book the room", assignee: sam, due_on: "2026-10-20" });
const shipped = makeActionItem({ id: "i2", title: "Ship the fix", status: "done" });

let mark: MutationStateFixture;
let edit: MutationStateFixture;
let removal: MutationStateFixture;

function membership(overrides: Partial<BoardMembership> = {}): BoardMembership {
    return { people: [sam, ali], member: true, failure: null, retry: vi.fn(), ...overrides };
}

function onBoard(stage: BoardStage, overrides: Partial<BoardSnapshot> = {}): BoardSnapshot {
    return makeBoardSnapshot({
        board: makeBoardView({ stage, cards_hidden: false, closed: stage === "end" }),
        viewer: makeParticipantViewer(),
        hidden_counts: {},
        ...overrides,
    });
}

function renderPanel(snapshot: BoardSnapshot, writable = true, cardFocus = createCardFocus()) {
    return renderWithProviders(<ActionItemsPanel snapshot={snapshot} writable={writable} cardFocus={cardFocus} />);
}

function row(title: string) {
    return within(screen.getByRole("listitem", { name: title }));
}

beforeEach(() => {
    vi.clearAllMocks();
    mark = makeMutationState();
    edit = makeMutationState();
    removal = makeMutationState();
    hooks.useMarkActionItem.mockReturnValue(mark);
    hooks.useEditActionItem.mockReturnValue(edit);
    hooks.useDeleteActionItem.mockReturnValue(removal);
    hooks.useBoardMembership.mockReturnValue(membership());
});

describe("ActionItemsPanel", () => {
    it("stays away before the Actions stage while the board has no action items", () => {
        // when
        renderPanel(onBoard("discuss"));

        // then
        expect(screen.queryByRole("region", { name: /Action items/ })).not.toBeInTheDocument();
        expect(hooks.useBoardMembership).toHaveBeenCalledWith(expect.any(String), false);
    });

    it("is a list only in the Actions stage, pointing to a card's + Action button", () => {
        // when
        renderPanel(onBoard("actions"));

        // then
        const panel = screen.getByRole("region", { name: "Action items 0" });
        expect(panel).toHaveTextContent("No action items yet.");
        expect(panel).toHaveTextContent("Actions are added from a card's + Action button.");
        expect(within(panel).queryByRole("form")).not.toBeInTheDocument();
        expect(within(panel).queryByRole("combobox")).not.toBeInTheDocument();
        expect(panel).not.toHaveTextContent(/Make action item/);
    });

    it("shows the card an item is for, and reveals that card when chosen", async () => {
        // given
        const user = userEvent.setup();
        const cardFocus = createCardFocus();
        const onReveal = vi.fn();
        cardFocus.watch("k1", onReveal);
        const card = makeCard({ id: "k1", body: "Standups ran long", hidden: false, is_mine: undefined });
        renderPanel(
            onBoard("actions", {
                cards: [card],
                action_items: [makeActionItem({ title: "Book the room", source_card_id: "k1" })],
            }),
            true,
            cardFocus,
        );

        // when
        await user.click(row("Book the room").getByRole("button", { name: "From: Standups ran long" }));

        // then
        expect(onReveal).toHaveBeenCalledTimes(1);
    });

    it("reveals a merged card through the card it was merged into", async () => {
        // given
        const user = userEvent.setup();
        const cardFocus = createCardFocus();
        const onReveal = vi.fn();
        cardFocus.watch("k1", onReveal);
        renderPanel(
            onBoard("discuss", {
                cards: [
                    makeCard({ id: "k1", body: "Releases", hidden: false }),
                    makeCard({ id: "k2", body: "Deploys are slow", hidden: false, parent_card_id: "k1" }),
                ],
                action_items: [makeActionItem({ title: "Speed up deploys", source_card_id: "k2" })],
            }),
            true,
            cardFocus,
        );

        // when
        await user.click(row("Speed up deploys").getByRole("button", { name: "From: Deploys are slow" }));

        // then
        expect(onReveal).toHaveBeenCalledTimes(1);
    });

    it("shows nothing extra for an item with no card, or a card the viewer cannot see", () => {
        // when
        renderPanel(
            onBoard("discuss", {
                cards: [makeCard({ id: "k1", body: "Standups ran long", hidden: false })],
                action_items: [
                    makeActionItem({ id: "i1", title: "Book the room", source_card_id: null }),
                    makeActionItem({ id: "i2", title: "Ship the fix", source_card_id: "k9" }),
                ],
            }),
        );

        // then
        expect(screen.queryByText(/^From:/)).not.toBeInTheDocument();
        expect(row("Book the room").queryByRole("button")).not.toBeInTheDocument();
        expect(row("Ship the fix").queryByRole("button")).not.toBeInTheDocument();
    });

    it("lists each item with its assignee, due date and status, and marks it done or reopens it", async () => {
        // given
        const user = userEvent.setup();
        renderPanel(onBoard("actions", { action_items: [booked, shipped] }));

        // when
        await user.click(row("Book the room").getByRole("button", { name: "Mark done" }));
        await user.click(row("Ship the fix").getByRole("button", { name: "Reopen" }));

        // then
        expect(row("Book the room").getByText(/Due 20 Oct 2026/)).toBeInTheDocument();
        expect(row("Book the room").getByText("Sam")).toBeInTheDocument();
        expect(row("Ship the fix").getByText("Done")).toBeInTheDocument();
        expect(mark.mutate).toHaveBeenNthCalledWith(1, { itemId: "i1", status: "done" });
        expect(mark.mutate).toHaveBeenNthCalledWith(2, { itemId: "i2", status: "open" });
    });

    it("drops an item from its menu", async () => {
        // given
        const user = userEvent.setup();
        renderPanel(onBoard("actions", { action_items: [booked] }));

        // when
        await user.click(row("Book the room").getByRole("button", { name: "Action item options" }));
        await user.click(screen.getByRole("menuitem", { name: "Drop" }));

        // then
        expect(mark.mutate).toHaveBeenCalledWith({ itemId: "i1", status: "dropped" });
    });

    it("edits an item in a dialog", async () => {
        // given
        const user = userEvent.setup();
        renderPanel(onBoard("actions", { action_items: [booked] }));

        // when
        await user.click(row("Book the room").getByRole("button", { name: "Action item options" }));
        await user.click(screen.getByRole("menuitem", { name: "Edit action item" }));
        const dialog = within(screen.getByRole("dialog", { name: "Edit action item" }));
        await user.clear(dialog.getByRole("combobox", { name: "Action item" }));
        await user.type(dialog.getByRole("combobox", { name: "Action item" }), "Book the big room");
        await user.click(dialog.getByRole("button", { name: "Save action item" }));

        // then
        expect(edit.mutate).toHaveBeenCalledWith(
            { itemId: "i1", title: "Book the big room", assignee_user_id: "u1", due_on: "2026-10-20", mentions: [] },
            expect.anything(),
        );
    });

    it("lets a facilitator delete an item behind a confirmation, in any stage before End", async () => {
        // given
        const user = userEvent.setup();
        renderPanel(onBoard("discuss", { viewer: makeBoardViewer(), action_items: [booked] }));

        // when
        await user.click(row("Book the room").getByRole("button", { name: "Action item options" }));
        const options = screen.getAllByRole("menuitem").map(item => item.textContent);
        await user.click(screen.getByRole("menuitem", { name: "Delete action item" }));
        await user.click(
            within(screen.getByRole("dialog", { name: "Delete this action item?" })).getByRole("button", {
                name: "Delete action item",
            }),
        );

        // then
        expect(options).toEqual(["Delete action item"]);
        expect(removal.mutate).toHaveBeenCalledWith({ itemId: "i1" }, expect.anything());
    });

    it("lets a participant delete the item they created in the Actions stage, and only that one", async () => {
        // given
        const own = makeActionItem({ id: "i3", title: "Write the notes", is_mine: true });
        const user = userEvent.setup();
        renderPanel(onBoard("actions", { action_items: [booked, own] }));

        // when
        await user.click(row("Book the room").getByRole("button", { name: "Action item options" }));
        const theirOptions = screen.getAllByRole("menuitem").map(item => item.textContent);
        await user.keyboard("{Escape}");
        await user.click(row("Write the notes").getByRole("button", { name: "Action item options" }));
        await user.click(screen.getByRole("menuitem", { name: "Delete action item" }));
        await user.click(
            within(screen.getByRole("dialog", { name: "Delete this action item?" })).getByRole("button", {
                name: "Delete action item",
            }),
        );

        // then
        expect(theirOptions).not.toContain("Delete action item");
        expect(removal.mutate).toHaveBeenCalledWith({ itemId: "i3" }, expect.anything());
    });

    it("keeps a creator's own item undeletable outside the Actions stage", () => {
        // when
        renderPanel(
            onBoard("discuss", { action_items: [makeActionItem({ title: "Write the notes", is_mine: true })] }),
        );

        // then
        expect(row("Write the notes").queryByRole("button")).not.toBeInTheDocument();
    });

    it("keeps items read-only for a participant outside the Actions stage", () => {
        // when
        renderPanel(onBoard("discuss", { action_items: [booked] }));

        // then
        expect(row("Book the room").queryByRole("button")).not.toBeInTheDocument();
        expect(screen.queryByText(/\+ Action button/)).not.toBeInTheDocument();
    });

    it("lets an organisation member mark items after End, and nothing else", () => {
        // when
        renderPanel(onBoard("end", { action_items: [booked] }));

        // then
        expect(row("Book the room").getByRole("button", { name: "Mark done" })).toBeInTheDocument();
        expect(row("Book the room").queryByRole("button", { name: "Action item options" })).toBeInTheDocument();
        expect(screen.queryByText(/\+ Action button/)).not.toBeInTheDocument();
    });

    it("leaves a guest's ended board read-only", () => {
        // given
        hooks.useBoardMembership.mockReturnValue(membership({ people: null, member: false }));

        // when
        renderPanel(onBoard("end", { action_items: [booked] }));

        // then
        expect(row("Book the room").queryByRole("button")).not.toBeInTheDocument();
    });

    it("offers nothing before the viewer has joined", () => {
        // when
        renderPanel(onBoard("actions", { action_items: [booked] }), false);

        // then
        expect(row("Book the room").queryByRole("button")).not.toBeInTheDocument();
        expect(screen.queryByText(/\+ Action button/)).not.toBeInTheDocument();
    });

    it("says why the people could not be read, and lets the viewer try again", async () => {
        // given
        const retry = vi.fn();
        hooks.useBoardMembership.mockReturnValue(
            membership({
                people: null,
                member: false,
                failure: makeApiError({ status: 503, message: "service unavailable" }),
                retry,
            }),
        );
        const user = userEvent.setup();
        renderPanel(onBoard("actions"));

        // when
        await user.click(screen.getByRole("button", { name: "Try again" }));

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("service unavailable");
        expect(retry).toHaveBeenCalledTimes(1);
    });

    it("shows why marking an item failed", () => {
        // given
        hooks.useMarkActionItem.mockReturnValue(
            makeMutationState({ error: makeApiError({ status: 409, message: "the board is read-only" }) }),
        );

        // when
        renderPanel(onBoard("actions", { action_items: [booked] }));

        // then
        expect(screen.getByText("the board is read-only")).toBeInTheDocument();
    });
});
