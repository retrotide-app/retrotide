import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import {
    makeActionItemList,
    makeActionItemListItem,
    makeApiError,
    makeMutationState,
    makeOrganisation,
    makePerson,
    makeQueryState,
    makeSessionUser,
    makeTeam,
    type MutationStateFixture,
} from "../../../test-utils/fixtures";
import { renderWithProviders } from "../../../test-utils/render";
import type { ActionItemListResponse, TeamListResponse } from "../../../types/api";
import { OrgActionItemsPage } from "./OrgActionItemsPage";

const hooks = vi.hoisted(() => ({
    useOrganisationActionItems: vi.fn(),
    useTeams: vi.fn(),
    useMarkListedActionItem: vi.fn(),
}));

vi.mock("../../../hooks/queries/actionItems", () => ({ useOrganisationActionItems: hooks.useOrganisationActionItems }));
vi.mock("../../../hooks/queries/teams", () => ({ useTeams: hooks.useTeams }));
vi.mock("../../../hooks/mutations/actionItems", () => ({ useMarkListedActionItem: hooks.useMarkListedActionItem }));

const harbour = makeOrganisation({ id: "org-harbour", slug: "harbour", role: "member" });

const ali = makePerson({ user_id: "u-ali", display_name: "Ali Khan" });

const booked = makeActionItemListItem({
    id: "i1",
    title: "Book the room",
    assignee: ali,
    due_on: "2026-10-20",
    board: { id: "b1", title: "Sprint 12" },
    team_name: "Crew",
});

const orphan = makeActionItemListItem({
    id: "i2",
    title: "Tidy the wiki",
    status: "done",
    board: null,
    team_id: undefined,
    team_name: undefined,
});

let mark: MutationStateFixture;

function showItems(overrides: Partial<ActionItemListResponse> = {}) {
    hooks.useOrganisationActionItems.mockReturnValue(
        makeQueryState<ActionItemListResponse>({
            data: makeActionItemList({
                action_items: [booked, orphan],
                total: 2,
                counts: { open: 1, done: 2, dropped: 1 },
                completion_rate: 2 / 3,
                ...overrides,
            }),
        }),
    );
}

function renderPage() {
    return renderWithProviders(<OrgActionItemsPage />, {
        user: makeSessionUser(),
        org: { organisations: [harbour] },
    });
}

function row(title: string) {
    return within(screen.getByRole("listitem", { name: title }));
}

function lastQuery() {
    return hooks.useOrganisationActionItems.mock.lastCall?.[1];
}

beforeEach(() => {
    vi.clearAllMocks();
    mark = makeMutationState();
    hooks.useMarkListedActionItem.mockReturnValue(mark);
    hooks.useTeams.mockReturnValue(
        makeQueryState<TeamListResponse>({ data: { teams: [makeTeam({ id: "team-crew", name: "Crew" })] } }),
    );
    showItems();
});

describe("OrgActionItemsPage", () => {
    it("lists the open action items first, with the completion rate and the counts", () => {
        // when
        renderPage();

        // then
        expect(screen.getByRole("heading", { level: 1, name: "Action items" })).toBeInTheDocument();
        expect(screen.getByText("67% complete")).toBeInTheDocument();
        expect(screen.getByText("2 done, 1 open")).toBeInTheDocument();
        expect(lastQuery()).toEqual({
            status: "open",
            assignee: undefined,
            team: undefined,
            limit: 50,
            offset: 0,
        });
        expect(hooks.useOrganisationActionItems).toHaveBeenCalledWith("org-harbour", expect.anything());
    });

    it("shows each item's assignee, due date, board and team, with a link to the board", () => {
        // when
        renderPage();

        // then
        expect(row("Book the room").getByText("Ali Khan")).toBeInTheDocument();
        expect(row("Book the room").getByText(/Due 20 Oct 2026/)).toBeInTheDocument();
        expect(row("Book the room").getByRole("link", { name: "Sprint 12" })).toHaveAttribute(
            "href",
            "/o/harbour/boards/b1",
        );
        expect(row("Book the room").getByText("Crew")).toBeInTheDocument();
        expect(row("Tidy the wiki").getByText(/Board deleted/)).toBeInTheDocument();
    });

    it("marks an item through its own board, and offers nothing on an item whose board is gone", async () => {
        // given
        const user = userEvent.setup();
        renderPage();

        // when
        await user.click(row("Book the room").getByRole("button", { name: "Mark done" }));

        // then
        expect(mark.mutate).toHaveBeenCalledWith({ boardId: "b1", itemId: "i1", status: "done" });
        expect(row("Tidy the wiki").queryByRole("button")).not.toBeInTheDocument();
    });

    it("drops an item from its menu", async () => {
        // given
        const user = userEvent.setup();
        renderPage();

        // when
        await user.click(row("Book the room").getByRole("button", { name: "Action item options" }));
        await user.click(screen.getByRole("menuitem", { name: "Drop" }));

        // then
        expect(mark.mutate).toHaveBeenCalledWith({ boardId: "b1", itemId: "i1", status: "dropped" });
    });

    it("filters by status, assignee and boards, starting again from the first page", async () => {
        // given
        const user = userEvent.setup();
        renderPage();

        // when
        await user.click(screen.getByRole("button", { name: "Done (2)" }));
        await user.selectOptions(screen.getByRole("combobox", { name: "Assignee" }), "Ali Khan");
        await user.selectOptions(screen.getByRole("combobox", { name: "Boards" }), "Organisation boards");

        // then
        expect(lastQuery()).toEqual({
            status: "done",
            assignee: "u-ali",
            team: "organisation",
            limit: 50,
            offset: 0,
        });
    });

    it("offers the viewer, nobody and each team as filters", () => {
        // when
        renderPage();

        // then
        const assignees = within(screen.getByRole("combobox", { name: "Assignee" }));
        const boards = within(screen.getByRole("combobox", { name: "Boards" }));
        expect(assignees.getAllByRole("option").map(option => option.textContent)).toEqual([
            "Anyone",
            "Assigned to me",
            "Unassigned",
            "Ali Khan",
        ]);
        expect(boards.getAllByRole("option").map(option => option.textContent)).toEqual([
            "All boards",
            "Organisation boards",
            "Crew",
        ]);
    });

    it("pages through a long list", async () => {
        // given
        showItems({ total: 120 });
        const user = userEvent.setup();
        renderPage();

        // when
        await user.click(screen.getByRole("button", { name: "Next" }));

        // then
        expect(lastQuery()).toEqual(expect.objectContaining({ offset: 50 }));
    });

    it("says when nothing is open yet, and when nothing matches the filters", async () => {
        // given
        showItems({ action_items: [], total: 0 });
        const user = userEvent.setup();
        renderPage();

        // then
        expect(screen.getByText(/No open action items/)).toBeInTheDocument();

        // when
        await user.click(screen.getByRole("button", { name: "All (4)" }));

        // then
        expect(screen.getByText("No action items match these filters.")).toBeInTheDocument();
    });

    it("says why the list failed to load", () => {
        // given
        hooks.useOrganisationActionItems.mockReturnValue(
            makeQueryState<ActionItemListResponse>({ error: makeApiError({ status: 503, message: "unavailable" }) }),
        );

        // when
        renderPage();

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("unavailable");
    });

    it("shows why marking an item failed", () => {
        // given
        hooks.useMarkListedActionItem.mockReturnValue(
            makeMutationState({ error: makeApiError({ status: 409, message: "the board is read-only" }) }),
        );

        // when
        renderPage();

        // then
        expect(screen.getByText("the board is read-only")).toBeInTheDocument();
    });

    it("sends the owner of a deleted organisation to restore it", () => {
        // when
        renderWithProviders(<OrgActionItemsPage />, {
            user: makeSessionUser(),
            org: { organisations: [{ ...harbour, role: "owner", deleted_at: "2026-10-01T09:00:00Z" }] },
        });

        // then
        expect(hooks.useOrganisationActionItems).not.toHaveBeenCalled();
        expect(screen.getByRole("link", { name: "Go to organisation settings" })).toBeInTheDocument();
    });
});
