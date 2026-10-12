import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Route, Routes } from "react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";
import {
    makeApiError,
    makeBoardListItem,
    makeMutationState,
    makeOrganisation,
    makeQueryState,
    makeTeam,
} from "../../test-utils/fixtures";
import { renderWithProviders } from "../../test-utils/render";
import type { BoardListResponse, TeamResponse } from "../../types/api";
import { TeamSettingsMenu } from "./TeamSettingsMenu";

const { useRenameTeam, useArchiveTeam, useDeleteTeam, useTeamBoards } = vi.hoisted(() => ({
    useRenameTeam: vi.fn(),
    useArchiveTeam: vi.fn(),
    useDeleteTeam: vi.fn(),
    useTeamBoards: vi.fn(),
}));

vi.mock("../../hooks/mutations/teams", () => ({ useRenameTeam, useArchiveTeam, useDeleteTeam }));
vi.mock("../../hooks/queries/boards", () => ({ useTeamBoards }));

function showBoards(count: number) {
    const boards = Array.from({ length: count }, (_, index) =>
        makeBoardListItem({ id: `board-${index}`, team_id: "team-crew" }),
    );
    useTeamBoards.mockReturnValue(makeQueryState<BoardListResponse>({ data: { boards } }));
}

const harbour = makeOrganisation({ id: "org-harbour", slug: "harbour", name: "Harbour" });
const crew = makeTeam({ id: "team-crew", org_id: "org-harbour", name: "Crew" });

interface MenuOptions {
    team?: TeamResponse;
    renames?: boolean;
    manages?: boolean;
}

function renderMenu({ team = crew, renames = true, manages = true }: MenuOptions = {}) {
    return renderWithProviders(
        <Routes>
            <Route
                path="/o/harbour/teams/team-crew"
                element={<TeamSettingsMenu organisation={harbour} team={team} renames={renames} manages={manages} />}
            />
            <Route path="/o/harbour/teams" element={<h1>Teams list</h1>} />
        </Routes>,
        { route: "/o/harbour/teams/team-crew" },
    );
}

async function choose(item: string) {
    const user = userEvent.setup();
    await user.click(screen.getByRole("button", { name: "Team settings" }));
    await user.click(screen.getByRole("menuitem", { name: item }));

    return user;
}

beforeEach(() => {
    vi.clearAllMocks();
    useRenameTeam.mockReturnValue(makeMutationState());
    useArchiveTeam.mockReturnValue(makeMutationState());
    useDeleteTeam.mockReturnValue(makeMutationState());
    showBoards(0);
});

describe("TeamSettingsMenu", () => {
    it("offers rename, archive and delete to someone who may do all three", async () => {
        // given
        const user = userEvent.setup();
        renderMenu();

        // when
        await user.click(screen.getByRole("button", { name: "Team settings" }));

        // then
        const items = within(screen.getByRole("menu", { name: "Team settings" })).getAllByRole("menuitem");
        expect(items.map(item => item.textContent)).toEqual(["Rename", "Archive", "Delete"]);
    });

    it("shows no menu to someone who may change nothing", () => {
        // when
        renderMenu({ renames: false, manages: false });

        // then
        expect(screen.queryByRole("button", { name: "Team settings" })).not.toBeInTheDocument();
    });

    it("renames the team from a dialog that starts from its name", async () => {
        // given
        const rename = makeMutationState();
        useRenameTeam.mockReturnValue(rename);
        renderMenu();
        const user = await choose("Rename");

        // when
        const field = within(screen.getByRole("dialog", { name: "Rename Crew" })).getByLabelText("Team name");
        const before = (field as HTMLInputElement).value;
        await user.clear(field);
        await user.type(field, "Deck crew");
        await user.click(screen.getByRole("button", { name: "Save name" }));

        // then
        expect(before).toBe("Crew");
        expect(rename.mutate).toHaveBeenCalledExactlyOnceWith(
            { orgId: "org-harbour", teamId: "team-crew", name: "Deck crew" },
            expect.objectContaining({ onSuccess: expect.any(Function) }),
        );
    });

    it("archives the team once confirmed", async () => {
        // given
        const archive = makeMutationState();
        useArchiveTeam.mockReturnValue(archive);
        renderMenu();
        const user = await choose("Archive");

        // when
        await user.click(screen.getByRole("button", { name: "Archive team" }));

        // then
        expect(screen.getByRole("dialog", { name: "Archive Crew?" })).toHaveTextContent("becomes read-only");
        expect(archive.mutate).toHaveBeenCalledExactlyOnceWith(
            { orgId: "org-harbour", teamId: "team-crew" },
            expect.objectContaining({ onSuccess: expect.any(Function) }),
        );
    });

    it("deletes the team once confirmed and goes back to the teams list", async () => {
        // given
        const deletion = makeMutationState();
        deletion.mutate.mockImplementation((_target, options: { onSuccess: () => void }) => options.onSuccess());
        useDeleteTeam.mockReturnValue(deletion);
        renderMenu();
        const user = await choose("Delete");

        // when
        await user.click(screen.getByRole("button", { name: "Delete team" }));

        // then
        expect(deletion.mutate).toHaveBeenCalledWith(
            { orgId: "org-harbour", teamId: "team-crew" },
            expect.objectContaining({ onSuccess: expect.any(Function) }),
        );
        expect(screen.getByRole("heading", { name: "Teams list" })).toBeInTheDocument();
    });

    it("warns how many boards the delete takes with it", async () => {
        // given
        showBoards(3);
        renderMenu();

        // when
        await choose("Delete");

        // then
        expect(screen.getByRole("dialog", { name: "Delete Crew?" })).toHaveTextContent(
            "Deleting Crew also deletes its 3 boards and every card on them. You cannot undo this.",
        );
    });

    it("warns about a single board in the singular", async () => {
        // given
        showBoards(1);
        renderMenu();

        // when
        await choose("Delete");

        // then
        expect(screen.getByRole("dialog", { name: "Delete Crew?" })).toHaveTextContent(
            "Deleting Crew also deletes its 1 board and every card on it.",
        );
    });

    it("says only the team goes when it has no boards", async () => {
        // when
        renderMenu();
        await choose("Delete");

        // then
        expect(screen.getByRole("dialog", { name: "Delete Crew?" })).toHaveTextContent(
            "Crew and its member list are deleted. You cannot undo this.",
        );
    });

    it("shows why a delete was refused inside the dialog", async () => {
        // given
        useDeleteTeam.mockReturnValue(
            makeMutationState({
                isError: true,
                error: makeApiError({ status: 500, message: "failed to delete the team" }),
            }),
        );
        renderMenu();

        // when
        await choose("Delete");

        // then
        expect(within(screen.getByRole("dialog", { name: "Delete Crew?" })).getByRole("alert")).toHaveTextContent(
            "failed to delete the team",
        );
    });

    it("offers only delete for an archived team", async () => {
        // given
        const user = userEvent.setup();
        renderMenu({ team: makeTeam({ ...crew, archived_at: "2026-09-10T09:00:00Z" }) });

        // when
        await user.click(screen.getByRole("button", { name: "Team settings" }));

        // then
        expect(screen.getAllByRole("menuitem").map(item => item.textContent)).toEqual(["Delete"]);
    });

    it("says the team was renamed", () => {
        // given
        useRenameTeam.mockReturnValue(makeMutationState({ isSuccess: true }));

        // when
        renderMenu();

        // then
        expect(screen.getByRole("status")).toHaveTextContent("Team renamed.");
    });
});
