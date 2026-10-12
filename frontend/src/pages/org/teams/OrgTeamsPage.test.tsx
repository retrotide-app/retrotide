import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import {
    makeApiError,
    makeMutationState,
    makeOrganisation,
    makeOrgBoardListItem,
    makeQueryState,
    makeSessionUser,
    makeTeam,
} from "../../../test-utils/fixtures";
import { renderWithProviders } from "../../../test-utils/render";
import type {
    OrgBoardListResponse,
    OrgRole,
    TeamListResponse,
    TeamMemberListResponse,
    TeamResponse,
} from "../../../types/api";
import { OrgTeamsPage } from "./OrgTeamsPage";

const { useTeams, useTeamMembers, useOrganisationBoards, useCreateTeam } = vi.hoisted(() => ({
    useTeams: vi.fn(),
    useTeamMembers: vi.fn(),
    useOrganisationBoards: vi.fn(),
    useCreateTeam: vi.fn(),
}));

vi.mock("../../../hooks/queries/teams", () => ({ useTeams, useTeamMembers }));
vi.mock("../../../hooks/queries/boards", () => ({ useOrganisationBoards }));
vi.mock("../../../hooks/mutations/teams", () => ({ useCreateTeam }));

const harbour = makeOrganisation({ id: "org-harbour", slug: "harbour", name: "Harbour", role: "owner" });
const crew = makeTeam({ id: "team-crew", name: "Crew", role: "lead" });
const deck = makeTeam({ id: "team-deck", name: "Deck", role: undefined });
const galley = makeTeam({ id: "team-galley", name: "Galley", role: "member", archived_at: "2026-09-10T09:00:00Z" });

function showTeams(teams: TeamResponse[]) {
    useTeams.mockReturnValue(makeQueryState<TeamListResponse>({ data: { teams } }));
}

function renderPage(role: OrgRole = "owner", deletedAt?: string) {
    const current = makeOrganisation({ ...harbour, role, deleted_at: deletedAt });

    return renderWithProviders(<OrgTeamsPage />, {
        user: makeSessionUser(),
        org: { organisations: [current], current },
    });
}

function cardOf(name: string): HTMLElement {
    return screen.getByRole("article", { name });
}

function succeed(_variables: unknown, options?: { onSuccess?: () => void }) {
    options?.onSuccess?.();
}

beforeEach(() => {
    vi.clearAllMocks();
    document.title = "";
    showTeams([crew, deck, galley]);
    useTeamMembers.mockReturnValue(makeQueryState<TeamMemberListResponse>({ data: { members: [] } }));
    useOrganisationBoards.mockReturnValue(
        makeQueryState<OrgBoardListResponse>({
            data: {
                boards: [
                    makeOrgBoardListItem({ id: "board-1", team_id: "team-crew" }),
                    makeOrgBoardListItem({ id: "board-2", team_id: "team-crew" }),
                ],
            },
        }),
    );
    useCreateTeam.mockReturnValue(makeMutationState());
});

describe("OrgTeamsPage", () => {
    it("heads the page and asks for the current organisation's teams and boards", () => {
        // when
        renderPage();

        // then
        expect(screen.getByRole("heading", { level: 1, name: "Teams" })).toBeInTheDocument();
        expect(document.title).toContain("Teams");
        expect(useTeams).toHaveBeenCalledWith("org-harbour");
        expect(useOrganisationBoards).toHaveBeenCalledWith("org-harbour");
    });

    it("shows each team as a card in a grid", () => {
        // when
        renderPage();

        // then
        const grid = screen.getByRole("list", { name: "Teams in Harbour" });
        expect(within(grid).getAllByRole("article")).toHaveLength(3);
        expect(within(cardOf("Galley")).getByText("Archived")).toBeInTheDocument();
    });

    it("counts each team's boards from the organisation's board list", () => {
        // when
        renderPage();

        // then
        expect(within(cardOf("Crew")).getByText("2 boards")).toBeInTheDocument();
        expect(within(cardOf("Galley")).getByText("0 boards")).toBeInTheDocument();
    });

    it("says it is loading while the teams are on their way", () => {
        // given
        useTeams.mockReturnValue(makeQueryState<TeamListResponse>({ loading: true }));

        // when
        renderPage();

        // then
        expect(screen.getByText("Loading teams...")).toBeInTheDocument();
    });

    it("says so when the organisation has no teams", () => {
        // given
        showTeams([]);

        // when
        renderPage();

        // then
        expect(screen.getByText("No teams yet.")).toBeInTheDocument();
        expect(screen.queryByRole("list", { name: "Teams in Harbour" })).not.toBeInTheDocument();
    });

    it("tells a plain member who is on no team that an owner or admin adds them", () => {
        // given
        showTeams([]);

        // when
        renderPage("member");

        // then
        expect(
            screen.getByText("You're not on any teams yet. An owner or admin can add you to one."),
        ).toBeInTheDocument();
    });

    it("shows the server's reason when the teams could not load, and tries again", async () => {
        // given
        const user = userEvent.setup();
        const failed = makeQueryState<TeamListResponse>({
            error: makeApiError({ status: 500, message: "teams unavailable" }),
        });
        useTeams.mockReturnValue(failed);
        renderPage();

        // when
        await user.click(screen.getByRole("button", { name: "Try again" }));

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("teams unavailable");
        expect(failed.refetch).toHaveBeenCalledOnce();
    });

    it("shows the server's reason when the boards could not load, and leaves the counts out", () => {
        // given
        useOrganisationBoards.mockReturnValue(
            makeQueryState<OrgBoardListResponse>({
                error: makeApiError({ status: 500, message: "boards unavailable" }),
            }),
        );

        // when
        renderPage();

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("boards unavailable");
        expect(within(cardOf("Crew")).queryByText("Boards")).not.toBeInTheDocument();
    });

    it("says the organisation is deleted instead of asking for its teams", () => {
        // when
        renderPage("owner", "2026-09-20T12:00:00Z");

        // then
        expect(screen.getByRole("heading", { level: 1, name: "Harbour is deleted" })).toBeInTheDocument();
        expect(useTeams).not.toHaveBeenCalled();
    });
});

describe("OrgTeamsPage creating a team", () => {
    it("creates a team from the new team dialog, closes it and says so", async () => {
        // given
        const user = userEvent.setup();
        const creation = makeMutationState();
        creation.mutate.mockImplementation(succeed);
        useCreateTeam.mockReturnValue(creation);
        const { rerender } = renderPage();

        // when
        await user.click(screen.getByRole("button", { name: "New team" }));
        await user.type(within(screen.getByRole("dialog", { name: "New team" })).getByLabelText("Team name"), "Deck");
        await user.click(screen.getByRole("button", { name: "Create team" }));
        useCreateTeam.mockReturnValue(makeMutationState({ isSuccess: true, data: makeTeam({ name: "Deck" }) }));
        rerender(<OrgTeamsPage />);

        // then
        expect(creation.mutate).toHaveBeenCalledWith(
            { orgId: "org-harbour", name: "Deck" },
            expect.objectContaining({ onSuccess: expect.any(Function) }),
        );
        expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
        expect(screen.getByRole("status")).toHaveTextContent("Deck created.");
    });

    it("keeps the dialog open with the server's reason when the name is refused", async () => {
        // given
        const user = userEvent.setup();
        useCreateTeam.mockReturnValue(
            makeMutationState({
                isError: true,
                error: makeApiError({ status: 409, message: "That name is taken.", code: "team_name_taken" }),
            }),
        );
        renderPage();

        // when
        await user.click(screen.getByRole("button", { name: "New team" }));

        // then
        const dialog = screen.getByRole("dialog", { name: "New team" });
        expect(within(dialog).getByLabelText("Team name")).toBeInvalid();
    });

    it("offers an admin the new team button", () => {
        // when
        renderPage("admin");

        // then
        expect(screen.getByRole("button", { name: "New team" })).toBeInTheDocument();
    });

    it("offers a member no way to create a team", () => {
        // when
        renderPage("member");

        // then
        expect(screen.queryByRole("button", { name: "New team" })).not.toBeInTheDocument();
    });

    it("leaves archive and delete to the team page", () => {
        // when
        renderPage("owner");

        // then
        expect(screen.queryByRole("button", { name: /archive/i })).not.toBeInTheDocument();
        expect(screen.queryByRole("button", { name: /delete/i })).not.toBeInTheDocument();
    });
});
