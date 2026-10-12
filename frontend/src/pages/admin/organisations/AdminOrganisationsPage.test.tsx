import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { StaffPermission } from "../../../domain/permissions";
import {
    makeAdminOrganisation,
    makeAdminOrganisationList,
    makeApiError,
    makeQueryState,
    makeSessionUser,
} from "../../../test-utils/fixtures";
import { renderWithProviders } from "../../../test-utils/render";
import type { AdminOrganisationListResponse } from "../../../types/api";
import { AdminOrganisationsPage } from "./AdminOrganisationsPage";

const { useAdminOrganisations } = vi.hoisted(() => ({ useAdminOrganisations: vi.fn() }));

vi.mock("../../../hooks/queries/adminOrganisations", () => ({ useAdminOrganisations }));

const harbour = makeAdminOrganisation({
    id: "org-harbour",
    name: "Harbour",
    slug: "harbour",
    owner_id: "user-robin",
    owner_display_name: "Robin",
    owner_email: "robin@example.test",
    member_count: 4,
    team_count: 2,
    board_count: 7,
});

function renderPage(permissions: StaffPermission[] = ["view_admin_panel", "view_organisations", "view_users"]) {
    return renderWithProviders(<AdminOrganisationsPage />, { user: makeSessionUser(), permissions });
}

beforeEach(() => {
    vi.clearAllMocks();
    useAdminOrganisations.mockReturnValue(
        makeQueryState<AdminOrganisationListResponse>({ data: makeAdminOrganisationList([harbour]) }),
    );
});

describe("AdminOrganisationsPage", () => {
    it("heads the page", () => {
        // given
        const title = "Organisations";

        // when
        renderPage();

        // then
        expect(screen.getByRole("heading", { level: 2, name: title })).toBeInTheDocument();
    });

    it("asks for the first page of every organisation", () => {
        // given
        const expected = { search: "", limit: 20, offset: 0 };

        // when
        renderPage();

        // then
        expect(useAdminOrganisations).toHaveBeenLastCalledWith(expected);
    });

    it("says it is loading while the first page is on its way", () => {
        // given
        useAdminOrganisations.mockReturnValue(makeQueryState<AdminOrganisationListResponse>({ loading: true }));

        // when
        renderPage();

        // then
        expect(screen.getByText("Loading organisations...")).toBeInTheDocument();
    });

    it("lists each organisation with a link to its record, its owner, plan, counts and status", () => {
        // given
        const quay = makeAdminOrganisation({
            id: "org-quay",
            name: "Quay",
            slug: "quay",
            owner_id: undefined,
            owner_display_name: undefined,
            owner_email: undefined,
            member_count: 0,
            team_count: 1,
            board_count: 0,
            deleted_at: "2026-09-20T10:00:00Z",
        });
        useAdminOrganisations.mockReturnValue(
            makeQueryState<AdminOrganisationListResponse>({ data: makeAdminOrganisationList([harbour, quay]) }),
        );

        // when
        renderPage();

        // then
        const table = screen.getByRole("table", { name: "Organisations" });
        const [, harbourRow, quayRow] = within(table).getAllByRole("row");
        expect(within(harbourRow).getByRole("link", { name: "Harbour" })).toHaveAttribute(
            "href",
            "/admin/organisations/org-harbour",
        );
        expect(within(harbourRow).getByRole("link", { name: "Robin" })).toHaveAttribute(
            "href",
            "/admin/users/user-robin",
        );
        const harbourCells = within(harbourRow).getAllByRole("cell");
        expect(harbourCells.map(cell => cell.textContent)).toEqual([
            "Harbourharbour",
            "Robinrobin@example.test",
            "Free",
            "4",
            "2",
            "7",
            expect.any(String),
            "Active",
        ]);
        expect(quayRow).toHaveTextContent("No owner");
        expect(quayRow).toHaveTextContent("Deleted");
        expect(quayRow).not.toHaveTextContent("Active");
    });

    it("names the owner without a link when the viewer may not see user records", () => {
        // when
        renderPage(["view_admin_panel", "view_organisations"]);

        // then
        const table = screen.getByRole("table", { name: "Organisations" });
        expect(within(table).queryByRole("link", { name: "Robin" })).not.toBeInTheDocument();
        expect(within(table).getByText("Robin")).toBeInTheDocument();
    });

    it("searches for what was typed, from the first page", async () => {
        // given
        const user = userEvent.setup();
        useAdminOrganisations.mockReturnValue(
            makeQueryState<AdminOrganisationListResponse>({
                data: makeAdminOrganisationList([harbour], { total: 45 }),
            }),
        );
        renderPage();
        await user.click(screen.getByRole("button", { name: "Next" }));

        // when
        await user.type(screen.getByRole("searchbox", { name: "Search organisations" }), "  harbour ");
        await user.click(screen.getByRole("button", { name: "Search" }));

        // then
        expect(useAdminOrganisations).toHaveBeenLastCalledWith({ search: "harbour", limit: 20, offset: 0 });
    });

    it("moves through the pages", async () => {
        // given
        const user = userEvent.setup();
        useAdminOrganisations.mockReturnValue(
            makeQueryState<AdminOrganisationListResponse>({
                data: makeAdminOrganisationList([harbour], { total: 45 }),
            }),
        );
        renderPage();

        // when
        await user.click(screen.getByRole("button", { name: "Next" }));

        // then
        expect(screen.getByRole("navigation", { name: "Organisation pages" })).toBeInTheDocument();
        expect(useAdminOrganisations).toHaveBeenLastCalledWith({ search: "", limit: 20, offset: 20 });
    });

    it("says so when there are no organisations", () => {
        // given
        useAdminOrganisations.mockReturnValue(
            makeQueryState<AdminOrganisationListResponse>({ data: makeAdminOrganisationList([]) }),
        );

        // when
        renderPage();

        // then
        expect(screen.getByText("No organisations yet.")).toBeInTheDocument();
        expect(screen.queryByRole("table", { name: "Organisations" })).not.toBeInTheDocument();
    });

    it("says so when nothing matches the search", async () => {
        // given
        const user = userEvent.setup();
        useAdminOrganisations.mockReturnValue(
            makeQueryState<AdminOrganisationListResponse>({ data: makeAdminOrganisationList([]) }),
        );
        renderPage();

        // when
        await user.type(screen.getByRole("searchbox", { name: "Search organisations" }), "nothing");
        await user.click(screen.getByRole("button", { name: "Search" }));

        // then
        expect(screen.getByText("No organisations match that search.")).toBeInTheDocument();
    });

    it("shows the server's reason when the organisations could not load, and tries again", async () => {
        // given
        const user = userEvent.setup();
        const failed = makeQueryState<AdminOrganisationListResponse>({
            error: makeApiError({ status: 500, message: "failed to list the organisations" }),
        });
        useAdminOrganisations.mockReturnValue(failed);
        renderPage();

        // when
        await user.click(screen.getByRole("button", { name: "Try again" }));

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("failed to list the organisations");
        expect(screen.queryByText("Loading organisations...")).not.toBeInTheDocument();
        expect(failed.refetch).toHaveBeenCalledOnce();
    });
});
