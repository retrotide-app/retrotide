import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { BoardMembership } from "../../../hooks/board/useBoardMembership";
import {
    makeApiError,
    makeBoardSnapshot,
    makeBoardView,
    makeMutationState,
    makeParticipantViewer,
    makePerson,
    makePreviousAction,
    type MutationStateFixture,
} from "../../../test-utils/fixtures";
import { renderWithProviders } from "../../../test-utils/render";
import type { BoardSnapshot } from "../../../types/api";
import { PreviousActionsPanel } from "./PreviousActionsPanel";

const hooks = vi.hoisted(() => ({
    useReviewPreviousAction: vi.fn(),
    useBoardMembership: vi.fn(),
}));

vi.mock("../../../hooks/mutations/actionItems", () => ({ useReviewPreviousAction: hooks.useReviewPreviousAction }));
vi.mock("../../../hooks/board/useBoardMembership", () => ({ useBoardMembership: hooks.useBoardMembership }));

const notes = makePreviousAction({
    id: "p1",
    title: "Write the release notes",
    assignee: makePerson({ display_name: "Ali Khan" }),
});
const retro = makePreviousAction({ id: "p2", title: "Move the retro", review: "carried" });

let review: MutationStateFixture;

function membership(overrides: Partial<BoardMembership> = {}): BoardMembership {
    return { people: [], member: true, failure: null, retry: vi.fn(), ...overrides };
}

function teamBoard(overrides: Partial<BoardSnapshot> = {}): BoardSnapshot {
    return makeBoardSnapshot({
        board: makeBoardView({ stage: "write" }),
        viewer: makeParticipantViewer(),
        previous_actions: [notes, retro],
        ...overrides,
    });
}

function renderPanel(snapshot: BoardSnapshot, writable = true) {
    return renderWithProviders(<PreviousActionsPanel snapshot={snapshot} writable={writable} />);
}

function row(title: string) {
    return within(screen.getByRole("listitem", { name: title }));
}

beforeEach(() => {
    vi.clearAllMocks();
    review = makeMutationState();
    hooks.useReviewPreviousAction.mockReturnValue(review);
    hooks.useBoardMembership.mockReturnValue(membership());
});

describe("PreviousActionsPanel", () => {
    it("shows the team's open items from earlier retros, with what was decided about each", () => {
        // when
        renderPanel(teamBoard());

        // then
        const panel = screen.getByRole("region", { name: "Previous actions 2" });
        expect(panel).toHaveTextContent("Open action items from this team's earlier retros.");
        expect(row("Write the release notes").getByText("Ali Khan")).toBeInTheDocument();
        expect(row("Write the release notes").getByText("Open")).toBeInTheDocument();
        expect(row("Move the retro").getByText("Carried over")).toBeInTheDocument();
    });

    it("records a member's review of an item, in any stage before End", async () => {
        // given
        const user = userEvent.setup();
        renderPanel(teamBoard());

        // when
        await user.click(row("Write the release notes").getByRole("button", { name: "Done" }));
        await user.click(row("Move the retro").getByRole("button", { name: "Drop" }));

        // then
        expect(review.mutate).toHaveBeenNthCalledWith(1, { itemId: "p1", outcome: "done" });
        expect(review.mutate).toHaveBeenNthCalledWith(2, { itemId: "p2", outcome: "dropped" });
    });

    it("marks the outcome already chosen, and sends nothing when it is chosen again", async () => {
        // given
        const user = userEvent.setup();
        renderPanel(teamBoard());

        // when
        const carry = row("Move the retro").getByRole("button", { name: "Carry over" });
        await user.click(carry);

        // then
        expect(carry).toHaveAttribute("aria-pressed", "true");
        expect(review.mutate).not.toHaveBeenCalled();
    });

    it("lets a guest see the list but never review it", () => {
        // given
        hooks.useBoardMembership.mockReturnValue(membership({ people: null, member: false }));

        // when
        renderPanel(teamBoard());

        // then
        expect(row("Write the release notes").queryByRole("button")).not.toBeInTheDocument();
    });

    it("says so once every item has been reviewed", () => {
        // when
        renderPanel(teamBoard({ previous_actions: [retro] }));

        // then
        expect(screen.getByRole("region", { name: "Previous actions 1" })).toHaveTextContent(
            "Every open action item from this team's earlier retros has been reviewed.",
        );
    });

    it("draws nothing when there is nothing to review", () => {
        // when
        renderPanel(teamBoard({ previous_actions: [] }));

        // then
        expect(screen.queryByRole("region", { name: /Previous actions/ })).not.toBeInTheDocument();
    });

    it("shows why a review failed", () => {
        // given
        hooks.useReviewPreviousAction.mockReturnValue(
            makeMutationState({ error: makeApiError({ status: 409, message: "the board is busy" }) }),
        );

        // when
        renderPanel(teamBoard());

        // then
        expect(screen.getByText("the board is busy")).toBeInTheDocument();
    });
});
