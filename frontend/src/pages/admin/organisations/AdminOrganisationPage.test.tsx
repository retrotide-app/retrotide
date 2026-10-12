import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { StaffPermission } from "../../../domain/permissions";
import {
    makeAdminOrganisationDetail,
    makeAdminOrganisationTeam,
    makeApiError,
    makeOrganisationMember,
    makeQueryState,
    makeSessionUser,
} from "../../../test-utils/fixtures";
import { renderWithProviders } from "../../../test-utils/render";
import type { AdminOrganisationDetailResponse } from "../../../types/api";
import { AdminOrganisationPage } from "./AdminOrganisationPage";

const { useAdminOrganisation } = vi.hoisted(() => ({ useAdminOrganisation: vi.fn() }));

vi.mock("../../../hooks/queries/adminOrganisations", () => ({ useAdminOrganisation }));

const harbour = makeAdminOrganisationDetail({
    id: "org-harbour",
    name: "Harbour",
    slug: "harbour",
    owner_id: "user-robin",
    owner_display_name: "Robin",
    owner_email: "robin@example.test",
    member_count: 2,
    team_count: 2,
    board_count: 6,
    teams: [
        makeAdminOrganisationTeam({ id: "team-crew", name: "Crew", member_count: 2, board_count: 5 }),
        makeAdminOrganisationTeam({
            id: "team-old",
            name: "Old crew",
            member_count: 1,
            board_count: 1,
            archived_at: "2026-09-10T10:00:00Z",
        }),
    ],
    members: [
        makeOrganisationMember({
            user_id: "user-robin",
            display_name: "Robin",
            email: "robin@example.test",
            role: "owner",
        }),
        makeOrganisationMember({
            user_id: "user-kit",
            display_name: "Kit",
            email: "kit@example.test",
            role: "member",
            site_role: "support",
        }),
    ],
});

function renderPage(permissions: StaffPermission[] = ["view_admin_panel", "view_organisations", "view_users"]) {
    return renderWithProviders(<AdminOrganisationPage />, {
        user: makeSessionUser(),
        permissions,
        route: "/admin/organisations/org-harbour",
        path: "/admin/organisations/:orgId",
    });
}

beforeEach(() => {
    vi.clearAllMocks();
    useAdminOrganisation.mockReturnValue(makeQueryState<AdminOrganisationDetailResponse>({ data: harbour }));
});

describe("AdminOrganisationPage", () => {
    it("asks for the organisation named in the address", () => {
        // when
        renderPage();

        // then
        expect(useAdminOrganisation).toHaveBeenLastCalledWith("org-harbour");
    });

    it("heads the page with the organisation's name and links back to the list", () => {
        // when
        renderPage();

        // then
        expect(screen.getByRole("heading", { level: 2, name: "Harbour" })).toBeInTheDocument();
        expect(screen.getByRole("link", { name: "Back to organisations" })).toHaveAttribute(
            "href",
            "/admin/organisations",
        );
    });

    it("says it is loading while the organisation is on its way", () => {
        // given
        useAdminOrganisation.mockReturnValue(makeQueryState<AdminOrganisationDetailResponse>({ loading: true }));

        // when
        renderPage();

        // then
        expect(screen.getByRole("heading", { level: 2, name: "Organisation" })).toBeInTheDocument();
        expect(screen.getByText("Loading organisation...")).toBeInTheDocument();
    });

    it("sums up the organisation's slug, plan, owner, status and counts", () => {
        // when
        renderPage();

        // then
        const details = screen.getByRole("region", { name: "Details" });
        expect(within(details).getByText("Slug").nextElementSibling).toHaveTextContent("harbour");
        expect(within(details).getByText("Plan").nextElementSibling).toHaveTextContent("Free");
        expect(within(details).getByText("Owner").nextElementSibling).toHaveTextContent("Robinrobin@example.test");
        expect(within(details).getByText("Status").nextElementSibling).toHaveTextContent("Active");
        expect(within(details).getByText("Members").nextElementSibling).toHaveTextContent("2");
        expect(within(details).getByText("Teams").nextElementSibling).toHaveTextContent("2");
        expect(within(details).getByText("Boards").nextElementSibling).toHaveTextContent("6");
        expect(within(details).queryByText("Deleted on")).not.toBeInTheDocument();
    });

    it("marks a deleted organisation and says when it was deleted", () => {
        // given
        useAdminOrganisation.mockReturnValue(
            makeQueryState<AdminOrganisationDetailResponse>({
                data: { ...harbour, deleted_at: "2026-09-20T10:00:00Z" },
            }),
        );

        // when
        renderPage();

        // then
        const details = screen.getByRole("region", { name: "Details" });
        expect(within(details).getByText("Status").nextElementSibling).toHaveTextContent("Deleted");
        expect(within(details).getByText("Deleted on")).toBeInTheDocument();
    });

    it("says when the organisation has no owner", () => {
        // given
        useAdminOrganisation.mockReturnValue(
            makeQueryState<AdminOrganisationDetailResponse>({
                data: { ...harbour, owner_id: undefined, owner_display_name: undefined, owner_email: undefined },
            }),
        );

        // when
        renderPage();

        // then
        const details = screen.getByRole("region", { name: "Details" });
        expect(within(details).getByText("Owner").nextElementSibling).toHaveTextContent("No owner");
    });

    it("lists each team with its counts and whether it is archived", () => {
        // when
        renderPage();

        // then
        const table = screen.getByRole("table", { name: "Teams" });
        const [, crewRow, oldRow] = within(table).getAllByRole("row");
        expect(crewRow).toHaveTextContent("Crew");
        expect(within(crewRow).getAllByRole("cell")[1]).toHaveTextContent("2");
        expect(within(crewRow).getAllByRole("cell")[2]).toHaveTextContent("5");
        expect(crewRow).toHaveTextContent("Active");
        expect(oldRow).toHaveTextContent("Old crew");
        expect(oldRow).toHaveTextContent("Archived");
    });

    it("lists each member with their role in the organisation and their site role", () => {
        // when
        renderPage();

        // then
        const table = screen.getByRole("table", { name: "Members" });
        const [, robinRow, kitRow] = within(table).getAllByRole("row");
        expect(within(robinRow).getByRole("link", { name: "Robin" })).toHaveAttribute(
            "href",
            "/admin/users/user-robin",
        );
        expect(robinRow).toHaveTextContent("robin@example.test");
        expect(robinRow).toHaveTextContent("Owner");
        expect(robinRow).toHaveTextContent("No role");
        expect(kitRow).toHaveTextContent("Member");
        expect(kitRow).toHaveTextContent("Support");
    });

    it("names members without links when the viewer may not see user records", () => {
        // when
        renderPage(["view_admin_panel", "view_organisations"]);

        // then
        const table = screen.getByRole("table", { name: "Members" });
        expect(within(table).queryByRole("link")).not.toBeInTheDocument();
        expect(within(table).getByText("Kit")).toBeInTheDocument();
    });

    it("says so when there are no teams or members", () => {
        // given
        useAdminOrganisation.mockReturnValue(
            makeQueryState<AdminOrganisationDetailResponse>({ data: { ...harbour, teams: [], members: [] } }),
        );

        // when
        renderPage();

        // then
        expect(screen.getByText("No teams yet.")).toBeInTheDocument();
        expect(screen.getByText("No members.")).toBeInTheDocument();
        expect(screen.queryByRole("table", { name: "Teams" })).not.toBeInTheDocument();
        expect(screen.queryByRole("table", { name: "Members" })).not.toBeInTheDocument();
    });

    it("offers nothing that changes the organisation", () => {
        // when
        renderPage();

        // then
        expect(screen.queryByRole("button")).not.toBeInTheDocument();
    });

    it("shows the server's reason when the organisation could not load, and tries again", async () => {
        // given
        const user = userEvent.setup();
        const failed = makeQueryState<AdminOrganisationDetailResponse>({
            error: makeApiError({ status: 404, message: "organisation not found" }),
        });
        useAdminOrganisation.mockReturnValue(failed);
        renderPage();

        // when
        await user.click(screen.getByRole("button", { name: "Try again" }));

        // then
        expect(screen.getByRole("alert")).toHaveTextContent("organisation not found");
        expect(screen.queryByText("Loading organisation...")).not.toBeInTheDocument();
        expect(failed.refetch).toHaveBeenCalledOnce();
    });
});
