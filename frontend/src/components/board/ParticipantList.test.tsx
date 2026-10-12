import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import {
    makeApiError,
    makeBoardParticipant,
    makeBoardSnapshot,
    makeMutationState,
    makeParticipantViewer,
} from "../../test-utils/fixtures";
import { renderWithProviders } from "../../test-utils/render";
import type { BoardSnapshot } from "../../types/api";
import { ParticipantList } from "./ParticipantList";

const { useSetParticipantRole, useRemoveParticipant } = vi.hoisted(() => ({
    useSetParticipantRole: vi.fn(),
    useRemoveParticipant: vi.fn(),
}));

vi.mock("../../hooks/mutations/boards", () => ({ useSetParticipantRole, useRemoveParticipant }));

const ali = makeBoardParticipant({ participant_id: "p2", display_name: "Ali", role: "participant" });

const bea = makeBoardParticipant({ participant_id: "p3", display_name: "Bea", role: "facilitator" });

const board = makeBoardSnapshot({ participants: [makeBoardParticipant(), ali, bea] });

let changeRole = makeMutationState();

let removal = makeMutationState();

function renderList(snapshot: BoardSnapshot = board) {
    return renderWithProviders(<ParticipantList snapshot={snapshot} />);
}

beforeEach(() => {
    vi.clearAllMocks();
    changeRole = makeMutationState();
    removal = makeMutationState();
    useSetParticipantRole.mockReturnValue(changeRole);
    useRemoveParticipant.mockReturnValue(removal);
});

describe("ParticipantList", () => {
    it("lists everyone with their role, facilitators first, and marks the viewer", () => {
        // when
        renderList();

        // then
        const rows = within(screen.getByRole("list", { name: "People on this board" })).getAllByRole("listitem");
        expect(rows.map(row => row.textContent)).toEqual([
            expect.stringContaining("BeaFacilitator"),
            expect.stringContaining("Sam (you)Facilitator"),
            expect.stringContaining("AliParticipant"),
        ]);
    });

    it("never offers the viewer actions on themselves", () => {
        // when
        renderList();

        // then
        expect(screen.queryByRole("button", { name: "Actions for Sam" })).not.toBeInTheDocument();
        expect(screen.getByRole("button", { name: "Actions for Ali" })).toBeInTheDocument();
    });

    it("makes a participant a co-facilitator", async () => {
        // given
        const user = userEvent.setup();
        renderList();

        // when
        await user.click(screen.getByRole("button", { name: "Actions for Ali" }));
        await user.click(screen.getByRole("menuitem", { name: "Make co-facilitator" }));

        // then
        expect(changeRole.mutate).toHaveBeenCalledWith({ participantId: "p2", role: "facilitator" });
    });

    it("makes a co-facilitator a participant again", async () => {
        // given
        const user = userEvent.setup();
        renderList();

        // when
        await user.click(screen.getByRole("button", { name: "Actions for Bea" }));
        await user.click(screen.getByRole("menuitem", { name: "Make participant" }));

        // then
        expect(changeRole.mutate).toHaveBeenCalledWith({ participantId: "p3", role: "participant" });
    });

    it("removes someone only once the facilitator confirms, and says their cards stay", async () => {
        // given
        const user = userEvent.setup();
        renderList();

        // when
        await user.click(screen.getByRole("button", { name: "Actions for Ali" }));
        await user.click(screen.getByRole("menuitem", { name: "Remove from board" }));
        const dialog = screen.getByRole("dialog", { name: "Remove Ali from the board?" });
        await user.click(within(dialog).getByRole("button", { name: "Remove" }));

        // then
        expect(dialog).toHaveTextContent("Their cards stay on the board.");
        expect(removal.mutate).toHaveBeenCalledWith({ participantId: "p2" }, expect.anything());
    });

    it("explains why a guest cannot be promoted", () => {
        // given
        useSetParticipantRole.mockReturnValue(
            makeMutationState({
                error: makeApiError({ status: 409, message: "cannot promote", code: "cannot_promote" }),
            }),
        );

        // when
        renderList();

        // then
        expect(screen.getByRole("status")).toHaveTextContent("Guests and people who joined by link alone stay");
    });

    it("explains why a team lead cannot be removed", async () => {
        // given
        useRemoveParticipant.mockReturnValue(
            makeMutationState({
                error: makeApiError({ status: 409, message: "lead facilitates", code: "lead_facilitates" }),
            }),
        );
        const user = userEvent.setup();
        renderList();

        // when
        await user.click(screen.getByRole("button", { name: "Actions for Bea" }));
        await user.click(screen.getByRole("menuitem", { name: "Remove from board" }));

        // then
        const dialog = screen.getByRole("dialog", { name: "Remove Bea from the board?" });
        expect(within(dialog).getByRole("alert")).toHaveTextContent(
            "This person facilitates through their team or organisation role",
        );
    });

    it("gives a participant the list with no actions", () => {
        // when
        renderList({ ...board, viewer: makeParticipantViewer() });

        // then
        expect(screen.getByRole("list", { name: "People on this board" })).toBeInTheDocument();
        expect(screen.queryByRole("button")).not.toBeInTheDocument();
    });
});
