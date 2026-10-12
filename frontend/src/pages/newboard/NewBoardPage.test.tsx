import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import {
    makeApiError,
    makeBoardTemplate,
    makeMutationState,
    makeOrganisation,
    makeQueryState,
    makeSavedTemplate,
    makeSessionUser,
    makeTeam,
} from "../../test-utils/fixtures";
import { renderWithProviders } from "../../test-utils/render";
import type {
    OrgRole,
    SavedTemplateListResponse,
    TeamListResponse,
    TeamResponse,
    TemplateListResponse,
} from "../../types/api";
import { NewBoardPage } from "./NewBoardPage";

const { useTeams, useCreateBoard, useBoardTemplates, useSavedTemplates } = vi.hoisted(() => ({
    useTeams: vi.fn(),
    useCreateBoard: vi.fn(),
    useBoardTemplates: vi.fn(),
    useSavedTemplates: vi.fn(),
}));

vi.mock("../../hooks/queries/teams", () => ({ useTeams }));
vi.mock("../../hooks/mutations/boards", () => ({ useCreateBoard }));
vi.mock("../../hooks/queries/boards", () => ({ useBoardTemplates }));
vi.mock("../../hooks/queries/savedTemplates", () => ({ useSavedTemplates }));

const NO_TEAM = "No team, everyone in the organisation";

const harbour = makeOrganisation({ id: "org-harbour", slug: "harbour", name: "Harbour" });
const crew = makeTeam({ id: "team-crew", org_id: "org-harbour", name: "Crew", role: "member" });
const deck = makeTeam({ id: "team-deck", org_id: "org-harbour", name: "Deck", role: "lead" });
const galley = makeTeam({ id: "team-galley", org_id: "org-harbour", name: "Galley", role: undefined });
const stores = makeTeam({
    id: "team-stores",
    org_id: "org-harbour",
    name: "Stores",
    role: "member",
    archived_at: "2026-09-10T09:00:00Z",
});

function showTeams(teams: TeamResponse[]) {
    useTeams.mockReturnValue(makeQueryState<TeamListResponse>({ data: { teams } }));
}

function renderPage(orgRole: OrgRole | null = "member", route = "/o/harbour/boards/new", deletedAt?: string) {
    const current = makeOrganisation({ ...harbour, role: orgRole ?? undefined, deleted_at: deletedAt });

    return renderWithProviders(<NewBoardPage />, {
        user: makeSessionUser(),
        org: { organisations: [current], current, role: orgRole },
        route,
        path: "/o/:orgSlug/boards/new",
    });
}

function teamOptions(): string[] {
    return within(screen.getByRole("combobox", { name: "Team" }))
        .getAllByRole("option")
        .map(option => option.textContent);
}

beforeEach(() => {
    vi.clearAllMocks();
    document.title = "";
    showTeams([crew, deck, galley, stores]);
    useCreateBoard.mockReturnValue(makeMutationState());
    useBoardTemplates.mockReturnValue(
        makeQueryState<TemplateListResponse>({ data: { templates: [makeBoardTemplate()] } }),
    );
    useSavedTemplates.mockReturnValue(makeQueryState<SavedTemplateListResponse>({ data: { templates: [] } }));
});

describe("NewBoardPage", () => {
    it("heads the page and asks for the organisation's teams", () => {
        // when
        renderPage();

        // then
        expect(screen.getByRole("heading", { level: 1, name: "New board" })).toBeInTheDocument();
        expect(useTeams).toHaveBeenCalledWith("org-harbour");
        expect(document.title).toContain("New board");
    });

    it("offers a plain member no team first, then only the live teams they belong to", () => {
        // when
        renderPage("member");

        // then
        expect(teamOptions()).toEqual([NO_TEAM, "Crew", "Deck"]);
        expect(screen.getByRole("combobox", { name: "Team" })).toHaveDisplayValue(NO_TEAM);
    });

    it("offers an admin no team and every live team", () => {
        // when
        renderPage("admin");

        // then
        expect(teamOptions()).toEqual([NO_TEAM, "Crew", "Deck", "Galley"]);
    });

    it("skips the team step for the team the address names", () => {
        // when
        renderPage("member", "/o/harbour/boards/new?team=team-deck");

        // then
        expect(screen.queryByRole("combobox", { name: "Team" })).not.toBeInTheDocument();
        expect(screen.getByRole("heading", { level: 2, name: "Pick a template" })).toBeInTheDocument();
    });

    it("skips the team step when the address asks for a board with no team", () => {
        // when
        renderPage("member", "/o/harbour/boards/new?team=none");

        // then
        expect(screen.queryByRole("combobox", { name: "Team" })).not.toBeInTheDocument();
        expect(screen.getByRole("heading", { level: 2, name: "Pick a template" })).toBeInTheDocument();
    });

    it("starts on the saved template the address names", async () => {
        // given
        const saved = makeSavedTemplate({ id: "saved-ours", name: "Our retro" });
        useSavedTemplates.mockReturnValue(makeQueryState<SavedTemplateListResponse>({ data: { templates: [saved] } }));
        const user = userEvent.setup();
        renderPage("member", "/o/harbour/boards/new?saved=saved-ours");

        // when
        await user.click(screen.getByRole("button", { name: "Next" }));

        // then
        expect(useSavedTemplates).toHaveBeenCalledWith("org-harbour");
        expect(screen.getByRole("radio", { name: "Our retro" })).toBeChecked();
    });

    it("asks for the team, starting on no team, when the address names one the viewer cannot create in", () => {
        // when
        renderPage("member", "/o/harbour/boards/new?team=team-stores");

        // then
        expect(teamOptions()).toEqual([NO_TEAM, "Crew", "Deck"]);
        expect(screen.getByRole("combobox", { name: "Team" })).toHaveDisplayValue(NO_TEAM);
    });

    it("still asks when the viewer can create in just one team, because no team is a choice too", () => {
        // given
        showTeams([crew, galley]);

        // when
        renderPage("member");

        // then
        expect(teamOptions()).toEqual([NO_TEAM, "Crew"]);
    });

    it("lets a member in no team create a board for the whole organisation", () => {
        // given
        showTeams([galley, stores]);

        // when
        renderPage("member");

        // then
        expect(teamOptions()).toEqual([NO_TEAM]);
        expect(screen.getByRole("list", { name: "Progress" })).toBeInTheDocument();
    });

    it("says a viewer with no organisation role cannot create boards, with no wizard", () => {
        // when
        renderPage(null);

        // then
        expect(screen.getByText("Your role in this organisation does not let you create boards.")).toBeInTheDocument();
        expect(screen.queryByRole("list", { name: "Progress" })).not.toBeInTheDocument();
    });

    it("says it is loading while the teams are on their way", () => {
        // given
        useTeams.mockReturnValue(makeQueryState<TeamListResponse>({ loading: true }));

        // when
        renderPage();

        // then
        expect(screen.getByText("Loading teams...")).toBeInTheDocument();
    });

    it("shows the server's reason when the teams could not load, and tries again", async () => {
        // given
        const failed = makeQueryState<TeamListResponse>({
            error: makeApiError({ status: 503, message: "session store unavailable" }),
        });
        useTeams.mockReturnValue(failed);
        const user = userEvent.setup();
        renderPage();

        // when
        await user.click(screen.getByRole("button", { name: "Try again" }));

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("session store unavailable");
        expect(failed.refetch).toHaveBeenCalledOnce();
        expect(screen.queryByText("Loading teams...")).not.toBeInTheDocument();
    });

    it("says the organisation is deleted instead of asking for its teams", () => {
        // when
        renderPage("owner", "/o/harbour/boards/new", "2026-09-30T09:00:00Z");

        // then
        expect(screen.getByRole("heading", { level: 1, name: "Harbour is deleted" })).toBeInTheDocument();
        expect(useTeams).not.toHaveBeenCalled();
        expect(screen.queryByRole("list", { name: "Progress" })).not.toBeInTheDocument();
    });
});
