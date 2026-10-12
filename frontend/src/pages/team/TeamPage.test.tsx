import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import {
    makeApiError,
    makeBoardListItem,
    makeMutationState,
    makeOrganisation,
    makeOrganisationMember,
    makeQueryState,
    makeSessionUser,
    makeTeam,
    makeTeamMember,
} from "../../test-utils/fixtures";
import { renderWithProviders } from "../../test-utils/render";
import type {
    BoardListResponse,
    OrganisationMemberListResponse,
    OrgRole,
    TeamMemberListResponse,
    TeamResponse,
    TeamRole,
} from "../../types/api";
import { TeamPage } from "./TeamPage";

const {
    useTeam,
    useTeamMembers,
    useOrganisationMembers,
    useRenameTeam,
    useArchiveTeam,
    useDeleteTeam,
    useChangeTeamMemberRole,
    useRemoveTeamMember,
    useAddTeamMember,
    useCreateInvite,
    useTeamBoards,
} = vi.hoisted(() => ({
    useTeamBoards: vi.fn(),
    useTeam: vi.fn(),
    useTeamMembers: vi.fn(),
    useOrganisationMembers: vi.fn(),
    useRenameTeam: vi.fn(),
    useArchiveTeam: vi.fn(),
    useDeleteTeam: vi.fn(),
    useChangeTeamMemberRole: vi.fn(),
    useRemoveTeamMember: vi.fn(),
    useAddTeamMember: vi.fn(),
    useCreateInvite: vi.fn(),
}));

vi.mock("../../hooks/queries/teams", () => ({ useTeam, useTeamMembers }));
vi.mock("../../hooks/queries/boards", () => ({ useTeamBoards }));
vi.mock("../../hooks/queries/organisations", () => ({ useOrganisationMembers }));
vi.mock("../../hooks/mutations/teams", () => ({
    useRenameTeam,
    useArchiveTeam,
    useDeleteTeam,
    useChangeTeamMemberRole,
    useRemoveTeamMember,
    useAddTeamMember,
}));
vi.mock("../../hooks/mutations/invites", () => ({ useCreateInvite }));

const viewer = makeSessionUser({ id: "user-sam", display_name: "Sam" });
const harbour = makeOrganisation({ id: "org-harbour", slug: "harbour", name: "Harbour", role: "owner" });
const crew = makeTeam({ id: "team-crew", org_id: "org-harbour", name: "Crew", role: "lead" });

function showTeam(team: TeamResponse) {
    useTeam.mockReturnValue(makeQueryState<TeamResponse>({ data: team }));
}

function renderPage(orgRole: OrgRole = "owner", deletedAt?: string) {
    const current = makeOrganisation({ ...harbour, role: orgRole, deleted_at: deletedAt });

    return renderWithProviders(<TeamPage />, {
        user: viewer,
        org: { organisations: [current], current },
        route: "/o/harbour/teams/team-crew",
        path: "/o/:orgSlug/teams/:teamId",
    });
}

function renderAsTeamMember(teamRole: TeamRole) {
    showTeam(makeTeam({ ...crew, role: teamRole }));

    return renderPage("member");
}

async function settingsItems(): Promise<string[]> {
    const user = userEvent.setup();
    await user.click(screen.getByRole("button", { name: "Team settings" }));

    const items = within(screen.getByRole("menu", { name: "Team settings" })).getAllByRole("menuitem");

    return items.map(item => item.textContent ?? "");
}

beforeEach(() => {
    vi.clearAllMocks();
    document.title = "";
    showTeam(crew);
    useTeamMembers.mockReturnValue(
        makeQueryState<TeamMemberListResponse>({
            data: { members: [makeTeamMember({ user_id: "user-robin", display_name: "Robin" })] },
        }),
    );
    useOrganisationMembers.mockReturnValue(
        makeQueryState<OrganisationMemberListResponse>({
            data: {
                members: [
                    makeOrganisationMember({ user_id: "user-sam", display_name: "Sam", role: "owner" }),
                    makeOrganisationMember({ user_id: "user-robin", display_name: "Robin", role: "member" }),
                ],
            },
        }),
    );
    useRenameTeam.mockReturnValue(makeMutationState());
    useArchiveTeam.mockReturnValue(makeMutationState());
    useDeleteTeam.mockReturnValue(makeMutationState());
    useChangeTeamMemberRole.mockReturnValue(makeMutationState());
    useRemoveTeamMember.mockReturnValue(makeMutationState());
    useAddTeamMember.mockReturnValue(makeMutationState());
    useCreateInvite.mockReturnValue(makeMutationState());
    useTeamBoards.mockReturnValue(
        makeQueryState<BoardListResponse>({
            data: { boards: [makeBoardListItem({ id: "board-sprint", title: "Sprint 12" })] },
        }),
    );
});

describe("TeamPage", () => {
    it("asks for the team the address names and heads the page with its name", () => {
        // when
        renderPage();

        // then
        expect(useTeam).toHaveBeenCalledWith("team-crew");
        expect(screen.getByRole("heading", { level: 1, name: "Crew" })).toBeInTheDocument();
        expect(document.title).toContain("Crew");
        expect(screen.getByRole("link", { name: "Back to teams" })).toHaveAttribute("href", "/o/harbour/teams");
        expect(screen.getByRole("list", { name: "Members of Crew" })).toHaveTextContent("Robin");
    });

    it("shows the team's boards as cards, each linking to its board", () => {
        // when
        renderPage();

        // then
        expect(useTeamBoards).toHaveBeenCalledWith("team-crew");
        const boards = screen.getByRole("list", { name: "Boards for Crew" });
        expect(within(boards).getByRole("link", { name: "Sprint 12" })).toHaveAttribute(
            "href",
            "/o/harbour/boards/board-sprint",
        );
    });

    it("says it is loading while the team is on its way", () => {
        // given
        useTeam.mockReturnValue(makeQueryState<TeamResponse>({ loading: true }));

        // when
        renderPage();

        // then
        expect(screen.getByRole("heading", { level: 1, name: "Team" })).toBeInTheDocument();
        expect(screen.getByText("Loading team...")).toBeInTheDocument();
    });

    it("shows the server's reason when the team could not load, and tries again", async () => {
        // given
        const user = userEvent.setup();
        const failed = makeQueryState<TeamResponse>({ error: makeApiError({ status: 403, message: "forbidden" }) });
        useTeam.mockReturnValue(failed);
        renderPage();

        // when
        await user.click(screen.getByRole("button", { name: "Try again" }));

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("forbidden");
        expect(screen.queryByText("Loading team...")).not.toBeInTheDocument();
        expect(failed.refetch).toHaveBeenCalledOnce();
    });

    it("keeps showing the team with the server's reason when a later read fails", () => {
        // given
        useTeam.mockReturnValue(
            makeQueryState<TeamResponse>({
                data: crew,
                error: makeApiError({ status: 404, message: "team not found" }),
            }),
        );

        // when
        renderPage();

        // then
        expect(screen.getByRole("heading", { level: 1, name: "Crew" })).toBeInTheDocument();
        expect(screen.getByRole("alert")).toHaveTextContent("team not found");
    });

    it("says the team is not found when it belongs to another organisation", () => {
        // given
        showTeam(makeTeam({ ...crew, org_id: "org-lighthouse" }));

        // when
        renderPage();

        // then
        expect(screen.getByRole("heading", { level: 1, name: "Team not found" })).toBeInTheDocument();
        expect(screen.getByText(/This team is not part of/)).toHaveTextContent("This team is not part of Harbour.");
        expect(screen.queryByRole("list", { name: "Members of Crew" })).not.toBeInTheDocument();
    });

    it("says the organisation is deleted instead of asking for the team", () => {
        // when
        renderPage("owner", "2026-09-20T12:00:00Z");

        // then
        expect(screen.getByRole("heading", { level: 1, name: "Harbour is deleted" })).toBeInTheDocument();
        expect(useTeam).not.toHaveBeenCalled();
    });
});

describe("TeamPage actions each viewer may take", () => {
    it("gives the owner a new board button, every team setting and the people controls", async () => {
        // when
        renderPage("owner");

        // then
        expect(screen.getByRole("link", { name: "New board" })).toHaveAttribute(
            "href",
            "/o/harbour/boards/new?team=team-crew",
        );
        expect(screen.getByRole("link", { name: "Add board" })).toHaveAttribute(
            "href",
            "/o/harbour/boards/new?team=team-crew",
        );
        expect(screen.getByRole("button", { name: "Add people" })).toBeInTheDocument();
        expect(screen.getByRole("button", { name: "Actions for Robin" })).toBeInTheDocument();
        expect(await settingsItems()).toEqual(["Rename", "Archive", "Delete"]);
    });

    it("lets a lead who is a plain organisation member rename the team, but not add people, archive or delete it", async () => {
        // when
        renderAsTeamMember("lead");

        // then only owners and admins see the organisation's people, so only they add them
        expect(screen.queryByRole("button", { name: "Add people" })).not.toBeInTheDocument();
        expect(await settingsItems()).toEqual(["Rename"]);
    });

    it("counts the viewer's site staff role, so a lead who is staff may make a member a lead", async () => {
        // given
        const user = userEvent.setup();
        const current = makeOrganisation({ ...harbour, role: "member" });
        showTeam(makeTeam({ ...crew, role: "lead" }));
        renderWithProviders(<TeamPage />, {
            user: viewer,
            auth: { siteRole: "support" },
            org: { organisations: [current], current },
            route: "/o/harbour/teams/team-crew",
            path: "/o/:orgSlug/teams/:teamId",
        });

        // when
        await user.click(screen.getByRole("button", { name: "Actions for Robin" }));

        // then
        expect(screen.getByRole("menuitem", { name: "Make lead" })).toBeInTheDocument();
    });

    it("keeps a lead who is not staff from making another lead", async () => {
        // given
        const user = userEvent.setup();
        renderAsTeamMember("lead");

        // when
        await user.click(screen.getByRole("button", { name: "Actions for Robin" }));

        // then
        expect(screen.queryByRole("menuitem", { name: "Make lead" })).not.toBeInTheDocument();
        expect(screen.getByRole("menuitem", { name: "Remove" })).toBeInTheDocument();
    });

    it("lets a team member see the team and add boards, but change nothing about it", () => {
        // when
        renderAsTeamMember("member");

        // then
        expect(screen.getByRole("list", { name: "Members of Crew" })).toBeInTheDocument();
        expect(screen.getByRole("link", { name: "New board" })).toBeInTheDocument();
        expect(screen.queryByRole("button", { name: "Team settings" })).not.toBeInTheDocument();
        expect(screen.queryByRole("button", { name: "Add people" })).not.toBeInTheDocument();
        expect(screen.queryByRole("button", { name: "Actions for Robin" })).not.toBeInTheDocument();
    });

    it("marks an archived team, lets nobody change it, and leaves the owner only delete", async () => {
        // given
        showTeam(makeTeam({ ...crew, archived_at: "2026-09-10T09:00:00Z" }));

        // when
        renderPage("owner");

        // then
        expect(screen.getAllByText("Archived").length).toBeGreaterThan(0);
        expect(
            screen.getByText("This team is archived, so nobody can rename it, change its members or add boards."),
        ).toBeInTheDocument();
        expect(screen.queryByRole("link", { name: "New board" })).not.toBeInTheDocument();
        expect(screen.queryByRole("link", { name: "Add board" })).not.toBeInTheDocument();
        expect(screen.queryByRole("button", { name: "Add people" })).not.toBeInTheDocument();
        expect(screen.queryByRole("button", { name: "Actions for Robin" })).not.toBeInTheDocument();
        expect(await settingsItems()).toEqual(["Delete"]);
    });
});
