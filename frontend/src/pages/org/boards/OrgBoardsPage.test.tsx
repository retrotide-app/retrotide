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
    DeletedBoardListResponse,
    OrgBoardListItem,
    OrgBoardListResponse,
    OrgRole,
    TeamListResponse,
    TeamResponse,
} from "../../../types/api";
import { OrgBoardsPage } from "./OrgBoardsPage";

const { useOrganisationBoards, useTeams, management } = vi.hoisted(() => ({
    useOrganisationBoards: vi.fn(),
    useTeams: vi.fn(),
    management: {
        usePinBoard: vi.fn(),
        useBulkBoards: vi.fn(),
        useRestoreBoard: vi.fn(),
        useDeletedBoards: vi.fn(),
        useBulkExportUrl: vi.fn(),
    },
}));

vi.mock("../../../hooks/queries/boards", () => ({ useOrganisationBoards }));
vi.mock("../../../hooks/queries/teams", () => ({ useTeams }));
vi.mock("../../../hooks/mutations/boardManagement", () => ({
    usePinBoard: management.usePinBoard,
    useBulkBoards: management.useBulkBoards,
    useRestoreBoard: management.useRestoreBoard,
}));
vi.mock("../../../hooks/queries/boardManagement", () => ({
    useDeletedBoards: management.useDeletedBoards,
    useBulkExportUrl: management.useBulkExportUrl,
}));

const harbour = makeOrganisation({ id: "org-harbour", slug: "harbour", name: "Harbour", role: "owner" });

const crew = makeTeam({ id: "team-crew", name: "Crew", role: "member" });

const deck = makeTeam({ id: "team-deck", name: "Deck", role: undefined });

const townHall = makeOrgBoardListItem({
    id: "board-town-hall",
    team_id: undefined,
    team_name: undefined,
    title: "Town hall",
});

const sprint = makeOrgBoardListItem({
    id: "board-sprint",
    team_id: "team-crew",
    team_name: "Crew",
    title: "Sprint 12",
});

const planning = makeOrgBoardListItem({
    id: "board-planning",
    team_id: "team-crew",
    team_name: "Crew",
    title: "Planning",
});

const launch = makeOrgBoardListItem({
    id: "board-launch",
    team_id: "team-deck",
    team_name: "Deck",
    title: "Launch review",
});

const retired = makeOrgBoardListItem({
    id: "board-retired",
    team_id: "team-crew",
    team_name: "Crew",
    title: "Sprint 11",
    archived_at: "2026-09-30T09:00:00Z",
});

function showBoards(boards: OrgBoardListItem[]) {
    useOrganisationBoards.mockReturnValue(makeQueryState<OrgBoardListResponse>({ data: { boards } }));
}

function showTeams(teams: TeamResponse[]) {
    useTeams.mockReturnValue(makeQueryState<TeamListResponse>({ data: { teams } }));
}

function renderPage(role: OrgRole = "owner", deletedAt?: string) {
    const current = makeOrganisation({ ...harbour, role, deleted_at: deletedAt });

    return renderWithProviders(<OrgBoardsPage />, {
        user: makeSessionUser(),
        org: { organisations: [current], current, role },
    });
}

function sectionTitles(): string[] {
    return screen.getAllByRole("heading", { level: 2 }).map(heading => heading.textContent);
}

function boardTitles(section: string): string[] {
    return within(screen.getByRole("list", { name: section }))
        .queryAllByRole("heading", { level: 3 })
        .map(heading => heading.textContent);
}

function addTile(section: string): HTMLElement | null {
    return within(screen.getByRole("region", { name: section })).queryByRole("link", { name: "Add board" });
}

beforeEach(() => {
    vi.clearAllMocks();
    document.title = "";
    showBoards([townHall, sprint, planning, launch, retired]);
    showTeams([crew, deck]);
    management.usePinBoard.mockReturnValue(makeMutationState());
    management.useBulkBoards.mockReturnValue(makeMutationState());
    management.useRestoreBoard.mockReturnValue(makeMutationState());
    management.useDeletedBoards.mockReturnValue(makeQueryState<DeletedBoardListResponse>({ data: { boards: [] } }));
    management.useBulkExportUrl.mockReturnValue("/api/v1/orgs/org-harbour/boards/export");
});

describe("OrgBoardsPage", () => {
    it("heads the page and asks for the current organisation's boards and teams", () => {
        // when
        renderPage();

        // then
        expect(screen.getByRole("heading", { level: 1, name: "Boards" })).toBeInTheDocument();
        expect(document.title).toContain("Boards");
        expect(useOrganisationBoards).toHaveBeenCalledWith("org-harbour");
        expect(useTeams).toHaveBeenCalledWith("org-harbour");
    });

    it("counts the live and the archived boards next to the title", () => {
        // when
        renderPage();

        // then
        expect(screen.getByText("4 boards")).toBeInTheDocument();
        expect(screen.getByText("1 archived")).toBeInTheDocument();
    });

    it("groups the boards into the organisation section first, then one section per team", () => {
        // when
        renderPage();

        // then
        expect(sectionTitles()).toEqual(["Organisation boards", "Crew", "Deck"]);
        expect(boardTitles("Organisation boards")).toEqual(["Town hall"]);
        expect(boardTitles("Crew")).toEqual(["Sprint 12", "Planning"]);
        expect(boardTitles("Deck")).toEqual(["Launch review"]);
    });

    it("says in one line who can open each section's boards", () => {
        // when
        renderPage();

        // then
        expect(screen.getByRole("region", { name: "Organisation boards" })).toHaveAccessibleDescription(
            "Everyone in the organisation can open these and add cards.",
        );
        expect(screen.getByRole("region", { name: "Crew" })).toHaveAccessibleDescription(
            "Only people on this team can open these.",
        );
    });

    it("opens the wizard from each Add board tile, with that section's team or with no team", () => {
        // when
        renderPage();

        // then
        const organisation = screen.getByRole("region", { name: "Organisation boards" });
        expect(within(organisation).getByRole("link", { name: "Add board" })).toHaveAttribute(
            "href",
            "/o/harbour/boards/new?team=none",
        );
        const team = screen.getByRole("region", { name: "Crew" });
        expect(within(team).getByRole("link", { name: "Add board" })).toHaveAttribute(
            "href",
            "/o/harbour/boards/new?team=team-crew",
        );
    });

    it("gives a member an organisation section and a section for their own team only", () => {
        // given
        showBoards([townHall, sprint]);

        // when
        renderPage("member");

        // then
        expect(sectionTitles()).toEqual(["Organisation boards", "Crew"]);
        expect(addTile("Organisation boards")).toBeInTheDocument();
        expect(addTile("Crew")).toBeInTheDocument();
    });

    it("filters the boards by title and drops the sections without a match", async () => {
        // given
        const user = userEvent.setup();
        renderPage();

        // when
        await user.type(screen.getByRole("searchbox", { name: "Filter your boards" }), "launch");

        // then
        expect(sectionTitles()).toEqual(["Deck"]);
        expect(boardTitles("Deck")).toEqual(["Launch review"]);
        expect(addTile("Deck")).not.toBeInTheDocument();
    });

    it("says when nothing matches the filter", async () => {
        // given
        const user = userEvent.setup();
        renderPage();

        // when
        await user.type(screen.getByRole("searchbox", { name: "Filter your boards" }), "zzz");

        // then
        expect(screen.getByText(/No boards match/)).toHaveTextContent("No boards match “zzz”.");
        expect(screen.queryAllByRole("heading", { level: 2 })).toEqual([]);
    });

    it("shows archived boards only when asked", async () => {
        // given
        const user = userEvent.setup();
        renderPage();
        expect(boardTitles("Crew")).not.toContain("Sprint 11");

        // when
        await user.selectOptions(screen.getByRole("combobox", { name: "Status" }), "Archived");

        // then
        expect(sectionTitles()).toEqual(["Crew"]);
        expect(boardTitles("Crew")).toEqual(["Sprint 11"]);
        expect(addTile("Crew")).not.toBeInTheDocument();
    });

    it("shows every board of the organisation, archived ones included, when asked for all", async () => {
        // given
        const user = userEvent.setup();
        renderPage();

        // when
        await user.selectOptions(screen.getByRole("combobox", { name: "Status" }), "All boards");

        // then
        expect(boardTitles("Crew")).toEqual(["Sprint 12", "Planning", "Sprint 11"]);
    });

    it("keeps only the chosen team's boards", async () => {
        // given
        const user = userEvent.setup();
        renderPage();

        // when
        await user.selectOptions(screen.getByRole("combobox", { name: "Team" }), "Deck");

        // then
        expect(sectionTitles()).toEqual(["Deck"]);
        expect(boardTitles("Deck")).toEqual(["Launch review"]);
    });

    it("says when no board matches the filters", async () => {
        // given
        showBoards([townHall, sprint]);
        const user = userEvent.setup();
        renderPage();

        // when
        await user.click(screen.getByRole("switch", { name: "Pinned only" }));

        // then
        expect(screen.getByText("No boards match these filters.")).toBeInTheDocument();
    });

    it("leads with the pinned boards, which stay in their team's section too", () => {
        // given
        showBoards([townHall, { ...sprint, pinned: true }, launch]);

        // when
        renderPage();

        // then
        expect(sectionTitles()).toEqual(["Pinned", "Organisation boards", "Crew", "Deck"]);
        expect(boardTitles("Pinned")).toEqual(["Sprint 12"]);
        expect(boardTitles("Crew")).toEqual(["Sprint 12"]);
    });

    it("pins a board from its tile", async () => {
        // given
        const pin = makeMutationState();
        management.usePinBoard.mockReturnValue(pin);
        const user = userEvent.setup();
        renderPage();

        // when
        const deckList = screen.getByRole("list", { name: "Deck" });
        await user.click(within(deckList).getByRole("button", { name: "Pin board" }));

        // then
        expect(management.usePinBoard).toHaveBeenCalledWith("org-harbour");
        expect(pin.mutate).toHaveBeenCalledExactlyOnceWith({ boardId: "board-launch", pinned: true });
    });

    it("unpins a pinned board", async () => {
        // given
        showBoards([{ ...launch, pinned: true }]);
        const pin = makeMutationState();
        management.usePinBoard.mockReturnValue(pin);
        const user = userEvent.setup();
        renderPage();

        // when
        const deckList = screen.getByRole("list", { name: "Deck" });
        await user.click(within(deckList).getByRole("button", { name: "Unpin board" }));

        // then
        expect(pin.mutate).toHaveBeenCalledExactlyOnceWith({ boardId: "board-launch", pinned: false });
    });

    it("chooses boards in selection mode and acts on them from the bulk bar", async () => {
        // given
        const bulk = makeMutationState();
        management.useBulkBoards.mockReturnValue(bulk);
        const user = userEvent.setup();
        renderPage();
        expect(screen.queryByRole("region", { name: "Selected boards" })).not.toBeInTheDocument();

        // when
        await user.click(screen.getByRole("button", { name: "Select" }));
        await user.click(screen.getByRole("checkbox", { name: "Select Sprint 12" }));
        await user.click(screen.getByRole("checkbox", { name: "Select Launch review" }));
        await user.click(
            within(screen.getByRole("region", { name: "Selected boards" })).getByRole("button", { name: "Archive" }),
        );

        // then
        expect(screen.getByRole("region", { name: "Selected boards" })).toHaveTextContent("2 boards selected");
        expect(bulk.mutate).toHaveBeenCalledWith(
            { action: "archive", boardIds: ["board-sprint", "board-launch"] },
            expect.anything(),
        );
    });

    it("drops the selection when selection mode ends", async () => {
        // given
        const user = userEvent.setup();
        renderPage();
        await user.click(screen.getByRole("button", { name: "Select" }));
        await user.click(screen.getByRole("checkbox", { name: "Select Sprint 12" }));

        // when
        await user.click(screen.getByRole("button", { name: "Select" }));
        await user.click(screen.getByRole("button", { name: "Select" }));

        // then
        expect(screen.getByRole("checkbox", { name: "Select Sprint 12" })).not.toBeChecked();
    });

    it("lists the recently deleted boards in place of the sections", async () => {
        // given
        management.useDeletedBoards.mockReturnValue(
            makeQueryState<DeletedBoardListResponse>({
                data: {
                    boards: [
                        {
                            id: "board-gone",
                            title: "Sprint 9",
                            deleted_at: "2026-10-09T09:00:00.000Z",
                            restore_until: "2026-11-08T09:00:00.000Z",
                        },
                    ],
                },
            }),
        );
        const user = userEvent.setup();
        renderPage();

        // when
        await user.selectOptions(screen.getByRole("combobox", { name: "Status" }), "Recently deleted");

        // then
        expect(sectionTitles()).toEqual(["Recently deleted"]);
        expect(screen.getByRole("button", { name: "Restore Sprint 9" })).toBeInTheDocument();
        expect(screen.queryByRole("button", { name: "Select" })).not.toBeInTheDocument();
    });

    it("puts the oldest boards first on request", async () => {
        // given
        const user = userEvent.setup();
        renderPage();

        // when
        await user.selectOptions(screen.getByRole("combobox", { name: "Sort" }), "Oldest first");

        // then
        expect(boardTitles("Crew")).toEqual(["Planning", "Sprint 12"]);
    });

    it("starts in the grid view and switches to the list view", async () => {
        // given
        const user = userEvent.setup();
        renderPage();
        expect(screen.getByRole("button", { name: "Grid" })).toHaveAttribute("aria-pressed", "true");

        // when
        await user.click(screen.getByRole("button", { name: "List" }));

        // then
        expect(screen.getByRole("button", { name: "List" })).toHaveAttribute("aria-pressed", "true");
        expect(boardTitles("Crew")).toEqual(["Sprint 12", "Planning"]);
    });

    it("says the boards are loading until they arrive, with no toolbar", () => {
        // given
        useOrganisationBoards.mockReturnValue(makeQueryState<OrgBoardListResponse>({ loading: true }));

        // when
        renderPage();

        // then
        expect(screen.getByText("Loading boards...")).toBeInTheDocument();
        expect(screen.queryByRole("searchbox", { name: "Filter your boards" })).not.toBeInTheDocument();
        expect(screen.queryByRole("region", { name: "No boards yet" })).not.toBeInTheDocument();
    });

    it("shows why the boards could not be read and asks again on request", async () => {
        // given
        const refetch = vi.fn().mockResolvedValue(undefined);
        useOrganisationBoards.mockReturnValue(
            makeQueryState<OrgBoardListResponse>({
                error: makeApiError({ status: 500, message: "failed to list the boards" }),
                refetch,
            }),
        );
        const user = userEvent.setup();
        renderPage();

        // when
        await user.click(screen.getByRole("button", { name: "Try again" }));

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("failed to list the boards");
        expect(refetch).toHaveBeenCalledOnce();
        expect(screen.queryByRole("region", { name: "No boards yet" })).not.toBeInTheDocument();
    });

    it("shows why the teams could not be read, and still lists the boards by the team they name", () => {
        // given
        useTeams.mockReturnValue(
            makeQueryState<TeamListResponse>({ error: makeApiError({ status: 500, message: "failed to list teams" }) }),
        );

        // when
        renderPage();

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("failed to list teams");
        expect(sectionTitles()).toEqual(["Organisation boards", "Crew", "Deck"]);
        expect(boardTitles("Crew")).toEqual(["Sprint 12", "Planning"]);
        expect(addTile("Organisation boards")).toBeInTheDocument();
        expect(addTile("Crew")).not.toBeInTheDocument();
    });

    it("shows the first board guide instead of empty sections, with no toolbar", () => {
        // given
        showBoards([]);

        // when
        renderPage("member");

        // then
        expect(screen.getByRole("region", { name: "No boards yet" })).toBeInTheDocument();
        expect(screen.getByRole("link", { name: "Create your first board" })).toHaveAttribute(
            "href",
            "/o/harbour/boards/new?team=none",
        );
        expect(screen.queryByRole("searchbox", { name: "Filter your boards" })).not.toBeInTheDocument();
        expect(screen.queryByText("0 boards")).not.toBeInTheDocument();
    });

    it("still reaches the recently deleted boards when no board is left", async () => {
        // given
        showBoards([]);
        const user = userEvent.setup();
        renderPage("member");

        // when
        await user.click(screen.getByRole("button", { name: "Recently deleted boards" }));

        // then
        expect(screen.getByRole("region", { name: "Recently deleted" })).toBeInTheDocument();
        expect(screen.getByRole("combobox", { name: "Status" })).toHaveValue("deleted");
        expect(screen.queryByRole("region", { name: "No boards yet" })).not.toBeInTheDocument();
    });

    it("sends the viewer to restore a deleted organisation instead", () => {
        // when
        renderPage("owner", "2026-10-01T09:00:00Z");

        // then
        expect(screen.getByRole("heading", { name: "Harbour is deleted" })).toBeInTheDocument();
        expect(useOrganisationBoards).not.toHaveBeenCalled();
    });
});
