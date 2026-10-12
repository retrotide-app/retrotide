import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Route, Routes } from "react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";
import {
    BOARD_ID,
    makeApiError,
    makeBoardSnapshot,
    makeBoardView,
    makeBoardViewer,
    makeMutationState,
    makeOrganisation,
    makeQueryState,
    makeTeam,
} from "../../test-utils/fixtures";
import { renderWithProviders } from "../../test-utils/render";
import type { BoardSnapshot, TeamListResponse } from "../../types/api";
import { BoardManagementSection } from "./BoardManagementSection";

const management = vi.hoisted(() => ({
    useArchiveBoard: vi.fn(),
    useUnarchiveBoard: vi.fn(),
    useDeleteBoard: vi.fn(),
    useMoveBoard: vi.fn(),
}));

const { useTeams } = vi.hoisted(() => ({ useTeams: vi.fn() }));

vi.mock("../../hooks/mutations/boardManagement", () => management);
vi.mock("../../hooks/queries/teams", () => ({ useTeams }));

const harbour = makeOrganisation({ id: "org-harbour", slug: "harbour", role: "admin" });

const crew = makeTeam({ id: "team-crew", name: "Crew", role: "member" });

const deck = makeTeam({ id: "team-deck", name: "Deck", role: undefined });

const teamBoard = makeBoardSnapshot({ board: makeBoardView({ team_id: "team-crew" }) });

function renderSection(snapshot: BoardSnapshot = teamBoard) {
    return renderWithProviders(
        <Routes>
            <Route
                path="/o/harbour/boards/:boardId/settings"
                element={<BoardManagementSection organisation={harbour} snapshot={snapshot} />}
            />
            <Route path="/o/harbour" element={<p>Boards page</p>} />
        </Routes>,
        { route: `/o/harbour/boards/${BOARD_ID}/settings` },
    );
}

function manage(): HTMLElement {
    return screen.getByRole("region", { name: "Manage board" });
}

beforeEach(() => {
    vi.clearAllMocks();
    for (const hook of Object.values(management)) {
        hook.mockReturnValue(makeMutationState());
    }
    useTeams.mockReturnValue(makeQueryState<TeamListResponse>({ data: { teams: [crew, deck] } }));
});

describe("BoardManagementSection", () => {
    it("offers to move, archive and delete the board", () => {
        // when
        renderSection();

        // then
        expect(within(manage()).getByRole("combobox", { name: "Team" })).toHaveValue("team-crew");
        expect(within(manage()).getByRole("button", { name: "Move board" })).toBeDisabled();
        expect(within(manage()).getByRole("button", { name: "Archive board" })).toBeInTheDocument();
        expect(within(manage()).getByRole("button", { name: "Delete board" })).toBeInTheDocument();
        expect(useTeams).toHaveBeenCalledWith("org-harbour");
        expect(management.useDeleteBoard).toHaveBeenCalledWith(BOARD_ID, "org-harbour");
    });

    it("lists the organisation and every team the viewer may create boards on", () => {
        // when
        renderSection();

        // then
        const options = within(within(manage()).getByRole("combobox", { name: "Team" }))
            .getAllByRole("option")
            .map(option => option.textContent);
        expect(options).toEqual(["Organisation board (no team)", "Crew", "Deck"]);
    });

    it("moves the board to the chosen team", async () => {
        // given
        const move = makeMutationState();
        management.useMoveBoard.mockReturnValue(move);
        const user = userEvent.setup();
        renderSection();

        // when
        await user.selectOptions(within(manage()).getByRole("combobox", { name: "Team" }), "Deck");
        await user.click(within(manage()).getByRole("button", { name: "Move board" }));

        // then
        expect(move.mutate).toHaveBeenCalledWith({ teamId: "team-deck" }, expect.anything());
    });

    it("moves the board to the organisation with no team", async () => {
        // given
        const move = makeMutationState();
        move.mutate.mockImplementation((_variables: unknown, options: { onSuccess: () => void }) => {
            options.onSuccess();
        });
        management.useMoveBoard.mockReturnValue(move);
        const user = userEvent.setup();
        renderSection();

        // when
        await user.selectOptions(
            within(manage()).getByRole("combobox", { name: "Team" }),
            "Organisation board (no team)",
        );
        await user.click(within(manage()).getByRole("button", { name: "Move board" }));

        // then
        expect(move.mutate).toHaveBeenCalledWith({ teamId: null }, expect.anything());
        expect(screen.getByRole("status")).toHaveTextContent("Board moved.");
    });

    it("archives a live board", async () => {
        // given
        const archive = makeMutationState();
        management.useArchiveBoard.mockReturnValue(archive);
        const user = userEvent.setup();
        renderSection();

        // when
        await user.click(within(manage()).getByRole("button", { name: "Archive board" }));

        // then
        expect(archive.mutate).toHaveBeenCalledOnce();
    });

    it("unarchives an archived board", async () => {
        // given
        const unarchive = makeMutationState();
        management.useUnarchiveBoard.mockReturnValue(unarchive);
        const user = userEvent.setup();
        renderSection(
            makeBoardSnapshot({ board: makeBoardView({ team_id: "team-crew", archived_at: "2026-10-02T09:00:00Z" }) }),
        );

        // when
        await user.click(within(manage()).getByRole("button", { name: "Unarchive board" }));

        // then
        expect(unarchive.mutate).toHaveBeenCalledOnce();
        expect(within(manage()).queryByRole("button", { name: "Archive board" })).not.toBeInTheDocument();
    });

    it("asks before deleting, names the 30 day restore window, then goes to the boards page", async () => {
        // given
        const remove = makeMutationState();
        remove.mutate.mockImplementation((_variables: unknown, options: { onSuccess: () => void }) => {
            options.onSuccess();
        });
        management.useDeleteBoard.mockReturnValue(remove);
        const user = userEvent.setup();
        renderSection();

        // when
        await user.click(within(manage()).getByRole("button", { name: "Delete board" }));
        const dialog = screen.getByRole("dialog", { name: "Delete this board?" });
        expect(dialog).toHaveTextContent("You can restore it from Recently deleted on the boards page for 30 days.");
        await user.click(within(dialog).getByRole("button", { name: "Delete board" }));

        // then
        expect(remove.mutate).toHaveBeenCalledOnce();
        expect(screen.getByText("Boards page")).toBeInTheDocument();
    });

    it("deletes nothing when the question is cancelled", async () => {
        // given
        const remove = makeMutationState();
        management.useDeleteBoard.mockReturnValue(remove);
        const user = userEvent.setup();
        renderSection();

        // when
        await user.click(within(manage()).getByRole("button", { name: "Delete board" }));
        await user.click(
            within(screen.getByRole("dialog", { name: "Delete this board?" })).getByRole("button", { name: "Cancel" }),
        );

        // then
        expect(remove.mutate).not.toHaveBeenCalled();
    });

    it("shows why a change was refused", () => {
        // given
        management.useMoveBoard.mockReturnValue(
            makeMutationState({
                isError: true,
                error: makeApiError({
                    status: 409,
                    message: "the team is archived, so it can no longer hold new boards",
                    code: "team_archived",
                }),
            }),
        );

        // when
        renderSection();

        // then
        expect(screen.getByRole("alert")).toHaveTextContent(
            "the team is archived, so it can no longer hold new boards",
        );
    });

    it("offers nothing to a viewer without board manage", () => {
        // when
        renderSection(
            makeBoardSnapshot({ viewer: makeBoardViewer({ capabilities: ["board.view", "board.participate"] }) }),
        );

        // then
        expect(within(manage()).queryByRole("button")).not.toBeInTheDocument();
        expect(within(manage()).queryByRole("combobox")).not.toBeInTheDocument();
    });
});
